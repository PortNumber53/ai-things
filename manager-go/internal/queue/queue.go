package queue

import (
	"net/url"

	amqp "github.com/rabbitmq/amqp091-go"
	"ai-things/manager-go/internal/utils"
)

type Client struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

type Message struct {
	Body []byte
	ack  func(bool) error
	nack func(bool, bool) error
}

func New(url string) (*Client, error) {
	utils.Logf("queue: connect %s", redactURL(url))
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Client{conn: conn, ch: ch}, nil
}

func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "<invalid url>"
	}
	if parsed.User == nil {
		return parsed.String()
	}
	username := parsed.User.Username()
	if _, hasPassword := parsed.User.Password(); hasPassword {
		parsed.User = url.UserPassword(username, "REDACTED")
	} else {
		parsed.User = url.User(username)
	}
	return parsed.String()
}

func (c *Client) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}

func (c *Client) ensureQueue(name string) error {
	utils.Logf("queue: ensure %s", name)
	_, err := c.ch.QueueDeclare(
		name,
		true,
		false,
		false,
		false,
		nil,
	)
	return err
}

func (c *Client) Publish(queueName string, payload []byte) error {
	utils.Logf("queue: publish queue=%s bytes=%d", queueName, len(payload))
	if err := c.ensureQueue(queueName); err != nil {
		return err
	}
	return c.ch.Publish(
		"",
		queueName,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        payload,
		},
	)
}

func (c *Client) Pop(queueName string) (*Message, error) {
	utils.Logf("queue: pop queue=%s", queueName)
	if err := c.ensureQueue(queueName); err != nil {
		return nil, err
	}
	msg, ok, err := c.ch.Get(queueName, false)
	if err != nil {
		return nil, err
	}
	if !ok {
		utils.Logf("queue: empty queue=%s", queueName)
		return nil, nil
	}
	utils.Logf("queue: received queue=%s bytes=%d", queueName, len(msg.Body))
	return &Message{
		Body: msg.Body,
		ack:  msg.Ack,
		nack: msg.Nack,
	}, nil
}

func (m *Message) Ack() error {
	if m == nil || m.ack == nil {
		return nil
	}
	utils.Logf("queue: ack")
	return m.ack(false)
}

func (m *Message) Nack(requeue bool) error {
	if m == nil || m.nack == nil {
		return nil
	}
	utils.Logf("queue: nack requeue=%t", requeue)
	return m.nack(false, requeue)
}
