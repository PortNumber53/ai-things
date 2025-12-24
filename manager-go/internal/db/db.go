package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ai-things/manager-go/internal/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type Content struct {
	ID        int64
	Title     string
	Status    *string
	Type      *string
	Sentences []byte
	Count     int
	Meta      []byte
	Archive   []byte
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Subscription struct {
	ID            int64
	FeedURL       string
	Title         *string
	Description   *string
	SiteURL       *string
	LastFetchedAt *time.Time
	LastBuildDate *time.Time
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Collection struct {
	ID          int64
	URL         string
	Title       string
	Language    string
	HTMLContent string
	FetchedAt   time.Time
	ProcessedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Subject struct {
	ID            int64
	Subject       string
	Keywords      *string
	IsActive      bool
	PodcastsCount int
	LastUsedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SlackInstallation struct {
	TeamID      string
	TeamName    string
	BotUserID   string
	BotToken    string
	Scope       string
	InstalledAt time.Time
	UpdatedAt   time.Time
}

type SlackThreadSession struct {
	TeamID            string
	ChannelID         string
	ThreadTS          string
	ActivatedByUserID string
	ActivatedAt       time.Time
	LastSeenAt        time.Time
	ExpiresAt         time.Time
}

func NewStore(ctx context.Context, connString string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) GetContentByID(ctx context.Context, id int64) (Content, error) {
	utils.Debug("db get content", "id", id)
	row := s.pool.QueryRow(ctx, `
		SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
		FROM contents
		WHERE id = $1
	`, id)

	var c Content
	err := row.Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.Type,
		&c.Sentences,
		&c.Count,
		&c.Meta,
		&c.Archive,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	return c, err
}

func (s *Store) FindFirstContent(ctx context.Context, where string, args ...any) (Content, error) {
	query := `
		SELECT id, title, status, type, sentences, count, meta, archive, created_at, updated_at
		FROM contents
		` + where + `
		ORDER BY id
		LIMIT 1
	`
	utils.Debug("db find first", "query", strings.TrimSpace(query), "args", args)
	row := s.pool.QueryRow(ctx, query, args...)
	var c Content
	err := row.Scan(
		&c.ID,
		&c.Title,
		&c.Status,
		&c.Type,
		&c.Sentences,
		&c.Count,
		&c.Meta,
		&c.Archive,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	return c, err
}

func (s *Store) CountContent(ctx context.Context, where string, args ...any) (int, error) {
	query := `SELECT COUNT(*) FROM contents ` + where
	utils.Debug("db count", "query", strings.TrimSpace(query), "args", args)
	row := s.pool.QueryRow(ctx, query, args...)
	var count int
	return count, row.Scan(&count)
}

func (s *Store) UpdateContentMetaStatus(ctx context.Context, id int64, status string, meta map[string]any) error {
	utils.Debug("db update meta+status", "id", id, "status", status)
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE contents
		SET status = $1,
			meta = $2,
			updated_at = NOW()
		WHERE id = $3
	`, status, metaJSON, id)
	return err
}

func (s *Store) UpdateContentMeta(ctx context.Context, id int64, meta map[string]any) error {
	utils.Debug("db update meta", "id", id)
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE contents
		SET meta = $1,
			updated_at = NOW()
		WHERE id = $2
	`, metaJSON, id)
	return err
}

func (s *Store) UpdateContentStatus(ctx context.Context, id int64, status string) error {
	utils.Debug("db update status", "id", id, "status", status)
	_, err := s.pool.Exec(ctx, `
		UPDATE contents
		SET status = $1,
			updated_at = NOW()
		WHERE id = $2
	`, status, id)
	return err
}

func (s *Store) UpdateContentType(ctx context.Context, id int64, contentType string) error {
	utils.Debug("db update type", "id", id, "type", contentType)
	_, err := s.pool.Exec(ctx, `
		UPDATE contents
		SET type = $1,
			updated_at = NOW()
		WHERE id = $2
	`, contentType, id)
	return err
}

func (s *Store) UpdateContentText(ctx context.Context, id int64, title string, sentences []byte, count int, meta []byte) error {
	utils.Debug("db update text payload", "id", id, "title_len", len(title), "count", count)
	_, err := s.pool.Exec(ctx, `
		UPDATE contents
		SET title = $1,
			sentences = $2,
			count = $3,
			meta = $4,
			updated_at = NOW()
		WHERE id = $5
	`, title, sentences, count, meta, id)
	return err
}

func (s *Store) UpdateContentArchive(ctx context.Context, id int64, archive []byte) error {
	utils.Debug("db update archive", "id", id, "bytes", len(archive))
	_, err := s.pool.Exec(ctx, `
		UPDATE contents
		SET archive = $1,
			updated_at = NOW()
		WHERE id = $2
	`, archive, id)
	return err
}

func (s *Store) CreateContent(ctx context.Context, content Content) (int64, error) {
	utils.Debug("db create content", "title_len", len(content.Title), "count", content.Count)
	row := s.pool.QueryRow(ctx, `
		INSERT INTO contents (title, status, type, sentences, count, meta, archive, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		RETURNING id
	`, content.Title, content.Status, content.Type, content.Sentences, content.Count, content.Meta, content.Archive)
	var id int64
	if err := row.Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) UpsertContentByID(ctx context.Context, content Content) error {
	if content.ID == 0 {
		return errors.New("missing content ID for upsert")
	}
	utils.Debug("db upsert content", "id", content.ID, "title_len", len(content.Title), "count", content.Count)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO contents (id, title, status, type, sentences, count, meta, archive, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		ON CONFLICT (id) DO UPDATE
		SET title = EXCLUDED.title,
			status = EXCLUDED.status,
			type = EXCLUDED.type,
			sentences = EXCLUDED.sentences,
			count = EXCLUDED.count,
			meta = EXCLUDED.meta,
			archive = COALESCE(EXCLUDED.archive, contents.archive),
			updated_at = NOW()
	`, content.ID, content.Title, content.Status, content.Type, content.Sentences, content.Count, content.Meta, content.Archive)
	return err
}

func (s *Store) QueryContents(ctx context.Context, query string, args ...any) ([]Content, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contents []Content
	for rows.Next() {
		var c Content
		if err := rows.Scan(
			&c.ID,
			&c.Title,
			&c.Status,
			&c.Type,
			&c.Sentences,
			&c.Count,
			&c.Meta,
			&c.Archive,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		contents = append(contents, c)
	}
	return contents, rows.Err()
}

func (s *Store) ListActiveSubscriptions(ctx context.Context) ([]Subscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, feed_url, title, description, site_url, last_fetched_at, last_build_date, is_active, created_at, updated_at
		FROM subscriptions
		WHERE is_active = true
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Subscription
	for rows.Next() {
		var ssub Subscription
		if err := rows.Scan(
			&ssub.ID,
			&ssub.FeedURL,
			&ssub.Title,
			&ssub.Description,
			&ssub.SiteURL,
			&ssub.LastFetchedAt,
			&ssub.LastBuildDate,
			&ssub.IsActive,
			&ssub.CreatedAt,
			&ssub.UpdatedAt,
		); err != nil {
			return nil, err
		}
		subs = append(subs, ssub)
	}
	return subs, rows.Err()
}

func (s *Store) InsertSubscription(ctx context.Context, sub Subscription) error {
	utils.Debug("db insert subscription", "url", sub.FeedURL)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subscriptions (feed_url, title, description, site_url, last_fetched_at, last_build_date, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`, sub.FeedURL, sub.Title, sub.Description, sub.SiteURL, sub.LastFetchedAt, sub.LastBuildDate, sub.IsActive)
	return err
}

func (s *Store) GetCollectionByURL(ctx context.Context, url string) (Collection, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, url, title, language, html_content, fetched_at, processed_at, created_at, updated_at
		FROM collections
		WHERE url = $1
	`, url)
	var c Collection
	err := row.Scan(
		&c.ID,
		&c.URL,
		&c.Title,
		&c.Language,
		&c.HTMLContent,
		&c.FetchedAt,
		&c.ProcessedAt,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Collection{}, nil
		}
		return Collection{}, err
	}
	return c, nil
}

func (s *Store) InsertCollection(ctx context.Context, c Collection) error {
	utils.Debug("db insert collection", "url", c.URL)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO collections (url, title, language, html_content, fetched_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`, c.URL, c.Title, c.Language, c.HTMLContent, c.FetchedAt)
	return err
}

func (s *Store) UpdateCollectionHTML(ctx context.Context, id int64, html string) error {
	utils.Debug("db update collection html", "id", id, "html_len", len(html))
	_, err := s.pool.Exec(ctx, `
		UPDATE collections
		SET html_content = $1,
			fetched_at = NOW(),
			updated_at = NOW()
		WHERE id = $2
	`, html, id)
	return err
}

func (s *Store) MarkCollectionProcessed(ctx context.Context, id int64) error {
	utils.Debug("db mark collection processed", "id", id)
	_, err := s.pool.Exec(ctx, `
		UPDATE collections
		SET processed_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

func (s *Store) ListCollectionsUnprocessed(ctx context.Context, lastID int64, limit int) ([]Collection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, url, title, language, html_content, fetched_at, processed_at, created_at, updated_at
		FROM collections
		WHERE processed_at IS NULL AND id > $1
		ORDER BY id
		LIMIT $2
	`, lastID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var c Collection
		if err := rows.Scan(
			&c.ID,
			&c.URL,
			&c.Title,
			&c.Language,
			&c.HTMLContent,
			&c.FetchedAt,
			&c.ProcessedAt,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	return collections, rows.Err()
}

func (s *Store) FindRandomSubject(ctx context.Context) (Subject, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, subject, keywords, is_active, podcasts_count, last_used_at, created_at, updated_at
		FROM subjects
		WHERE podcasts_count < 1 AND is_active = true
		ORDER BY random()
		LIMIT 1
	`)
	var subj Subject
	if err := row.Scan(
		&subj.ID,
		&subj.Subject,
		&subj.Keywords,
		&subj.IsActive,
		&subj.PodcastsCount,
		&subj.LastUsedAt,
		&subj.CreatedAt,
		&subj.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subject{}, nil
		}
		return Subject{}, err
	}
	return subj, nil
}

func (s *Store) GetSubjectByName(ctx context.Context, name string) (Subject, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, subject, keywords, is_active, podcasts_count, last_used_at, created_at, updated_at
		FROM subjects
		WHERE subject = $1
		LIMIT 1
	`, name)
	var subj Subject
	if err := row.Scan(
		&subj.ID,
		&subj.Subject,
		&subj.Keywords,
		&subj.IsActive,
		&subj.PodcastsCount,
		&subj.LastUsedAt,
		&subj.CreatedAt,
		&subj.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Subject{}, nil
		}
		return Subject{}, err
	}
	return subj, nil
}

func (s *Store) InsertSubject(ctx context.Context, name string) error {
	utils.Debug("db insert subject", "subject_len", len(name))
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subjects (subject, is_active, podcasts_count, created_at, updated_at)
		VALUES ($1, true, 0, NOW(), NOW())
	`, name)
	return err
}

func (s *Store) IncrementSubjectPodcasts(ctx context.Context, id int64) error {
	utils.Debug("db increment subject podcasts", "id", id)
	_, err := s.pool.Exec(ctx, `
		UPDATE subjects
		SET podcasts_count = podcasts_count + 1,
			last_used_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, id)
	return err
}

func (s *Store) UpsertSlackInstallation(ctx context.Context, inst SlackInstallation) error {
	utils.Debug("db upsert slack installation", "team_id", inst.TeamID)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO slack_installations (team_id, team_name, bot_user_id, bot_token, scope, installed_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (team_id) DO UPDATE SET
			team_name = EXCLUDED.team_name,
			bot_user_id = EXCLUDED.bot_user_id,
			bot_token = EXCLUDED.bot_token,
			scope = EXCLUDED.scope,
			updated_at = NOW()
	`, inst.TeamID, inst.TeamName, inst.BotUserID, inst.BotToken, inst.Scope)
	return err
}

func (s *Store) GetSlackBotToken(ctx context.Context, teamID string) (string, error) {
	utils.Debug("db get slack bot token", "team_id", teamID)
	var token string
	err := s.pool.QueryRow(ctx, `
		SELECT bot_token
		FROM slack_installations
		WHERE team_id = $1
	`, teamID).Scan(&token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return token, nil
}

func (s *Store) UpsertSlackThreadSession(ctx context.Context, teamID, channelID, threadTS, activatedByUserID string, ttl time.Duration) error {
	if teamID == "" || channelID == "" || threadTS == "" {
		return errors.New("missing teamID/channelID/threadTS")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	expiresAt := time.Now().Add(ttl)
	utils.Debug("db upsert slack thread session", "team_id", teamID, "channel", channelID, "thread_ts", threadTS)
	_, err := s.pool.Exec(ctx, `
		INSERT INTO slack_thread_sessions (team_id, channel_id, thread_ts, activated_by_user_id, activated_at, last_seen_at, expires_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW(), $5)
		ON CONFLICT (team_id, channel_id, thread_ts) DO UPDATE SET
			last_seen_at = NOW(),
			expires_at = GREATEST(slack_thread_sessions.expires_at, EXCLUDED.expires_at),
			activated_by_user_id = COALESCE(slack_thread_sessions.activated_by_user_id, EXCLUDED.activated_by_user_id)
	`, teamID, channelID, threadTS, activatedByUserID, expiresAt)
	return err
}

func (s *Store) IsSlackThreadSessionActive(ctx context.Context, teamID, channelID, threadTS string) (bool, error) {
	if teamID == "" || channelID == "" || threadTS == "" {
		return false, errors.New("missing teamID/channelID/threadTS")
	}
	utils.Debug("db slack thread session active?", "team_id", teamID, "channel", channelID, "thread_ts", threadTS)
	var active bool
	err := s.pool.QueryRow(ctx, `
		SELECT expires_at > NOW()
		FROM slack_thread_sessions
		WHERE team_id = $1 AND channel_id = $2 AND thread_ts = $3
	`, teamID, channelID, threadTS).Scan(&active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return active, nil
}

func StatusTrueCondition(flags []string) string {
	conds := make([]string, 0, len(flags))
	for _, flag := range flags {
		conds = append(conds, fmt.Sprintf("meta->'status'->>'%s' = 'true'", flag))
	}
	return strings.Join(conds, " AND ")
}

func StatusNotTrueCondition(flags []string) string {
	conds := make([]string, 0, len(flags))
	for _, flag := range flags {
		conds = append(conds, fmt.Sprintf("(meta->'status'->>'%s' IS NULL OR meta->'status'->>'%s' <> 'true')", flag, flag))
	}
	return strings.Join(conds, " AND ")
}

func StatusFalseCondition(flags []string) string {
	conds := make([]string, 0, len(flags))
	for _, flag := range flags {
		conds = append(conds, fmt.Sprintf("meta->'status'->>'%s' = 'false'", flag))
	}
	return strings.Join(conds, " AND ")
}

func MetaKeyMissingCondition(keys []string) string {
	conds := make([]string, 0, len(keys))
	for _, key := range keys {
		conds = append(conds, fmt.Sprintf("NOT (meta ? '%s')", key))
	}
	return strings.Join(conds, " AND ")
}
