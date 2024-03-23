<?php

namespace App\Console\Commands\Base;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

abstract class BaseJobCommand extends Command
{
    protected $queue;
    protected $content;

    protected $ignore_host_check = false;

    protected $queue_input  = 'queue_input';
    protected $queue_output = 'queue_output';

    protected $message_hostname;

    public function __construct(Content $content, Queue $queue)
    {
        parent::__construct();
        $this->content = $content;
        $this->queue = $queue;
    }

    public function handle()
    {
        try {
            $content_id = $this->argument('content_id');
            $sleep = $this->option('sleep');

            if (!$content_id) {
                $this->processQueueMessages($sleep);
            } else {
                $this->processContent($content_id);
            }
        } catch (\Exception $e) {
            Log::error($e->getMessage());
            $this->error('An error occurred. Please check the logs for more details.');
        }
    }

    protected function processQueueMessages($sleep)
    {
        while (true) {
            $this->line("Checking queue: " . $this->queue_input);
            $message = $this->queue->pop($this->queue_input);

            if ($message) {
                try {
                    $this->processMessage($message);
                } catch (\Exception $e) {
                    Log::error("Error processing message: " . $e->getMessage());
                    // Handle retries or logging based on your requirements
                }
            } else {
                $this->line("No message found, sleeping");
                sleep($sleep);
            }
        }
    }

    protected function processMessage($message)
    {
        $hostname = gethostname();
        $payload = json_decode($message->getRawBody(), true);

        if (!isset($payload['content_id'])) {
            Log::warning("Invalid message format: " . json_encode($payload));
            return;
        }

        $this->message_hostname = data_get($payload, 'hostname');
        if (!$this->ignore_host_check && $this->message_hostname !== $hostname) {
            $payload_hostname = $payload['hostname'];
            Log::info("[{$this->message_hostname}] - Message received on a different host [{$this->message_hostname}]. Re-queuing or ignoring.");
            // Handle re-queuing or ignoring the message based on your requirements
            return;
        }

        // $this->message_hostname = $payload['hostname'];
        $this->processContent($payload['content_id']);
        $message->delete();
    }
}
