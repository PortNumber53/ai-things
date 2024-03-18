<?php

namespace App\Console\Commands\Base;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

abstract class BaseJobCommand extends Command
{
    // protected $signature = 'job:BaseCommand
    //     {content_id? : The content ID}
    //     {--sleep=30 : Sleep time in seconds}
    //     ';
    // protected $description = 'Define some description';
    protected $queue;
    protected $content;

    protected const QUEUE_INPUT  = 'queue_input';
    protected const QUEUE_OUTPUT = 'queue_output';

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
            $message = $this->queue->pop(self::QUEUE_INPUT);

            if ($message) {
                $this->processMessage($message);
            }

            $this->line("No message found, sleeping");
            sleep($sleep);
        }
    }

    protected function processMessage($message)
    {
        $hostname = gethostname();
        try {
            $payload = json_decode($message->getRawBody(), true);

            if (isset($payload['content_id']) && isset($payload['hostname'])) {
                if ($payload['hostname'] === $hostname) {
                    $this->processContent($payload['content_id']);
                    $message->delete(); // Message processed on the correct host, delete it
                } else {
                    Log::info("[{$hostname}] - Message received on a different host. Re-queuing or ignoring.");
                    // You can re-queue the message here if needed
                    $this->queue->push(self::QUEUE_INPUT, $payload);
                    // Or you can simply ignore the message
                }
            }
        } catch (\Exception $e) {
            Log::error("Error processing message: " . $e->getMessage());
            // Handle the error, maybe retry or log
        }
    }

    // protected function processContent($content_id)
    // {
    //     // Your content processing logic here
    // }
}
