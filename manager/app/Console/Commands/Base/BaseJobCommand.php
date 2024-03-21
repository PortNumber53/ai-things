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
                $this->processMessage($message);
            } else {
                $this->line("No message found, sleeping");
                sleep($sleep);
            }
        }
    }

    protected function processMessage($message)
    {
        $hostname = gethostname();
        try {
            $payload = json_decode($message->getRawBody(), true);

            if (isset($payload['content_id']) && isset($payload['hostname'])) {
                if ($this->ignore_host_check || $payload['hostname'] === $hostname) {
                    $this->processContent($payload['content_id']);
                    $message->delete(); // Message processed on the correct host, delete it
                } else {
                    Log::info("[{$hostname}] - Message received on a different host. Re-queuing or ignoring.");
                    // You can re-queue the message here if needed
                    // $this->queue->pushRaw(json_encode($payload), $this->queue_input);
                    // Or you can simply ignore the message
                    // $message->delete(); // Message processed on the correct host, delete it
                }
            }
        } catch (\Exception $e) {
            Log::error("Error processing message: " . $e->getMessage());
            // Handle the error, maybe retry or log
        }
    }
}
