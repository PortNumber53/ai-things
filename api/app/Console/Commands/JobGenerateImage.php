<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

class JobGenerateImage extends Command
{
    protected $signature = 'job:GenerateImage
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';

    protected $description = 'Generate image for podcast';
    protected $content;
    protected $queue;


    public function __construct(Content $content, Queue $queue)
    {
        parent::__construct();
        $this->content = $content;
        $this->queue = $queue;
    }

    public function handle()
    {
        $content_id = $this->argument('content_id');
        $sleep = $this->option('sleep');

        if (!$content_id) {
            $this->processQueueMessage($sleep);
        }
        $this->processContent($content_id);
    }

    private function processQueueMessage($sleep)
    {
        while (true) {
            $message = $this->queue->pop('build_podcast');
            if ($message) {
                $payload = json_decode($message->getRawBody(), true);

                if (isset($payload['content_id']) && isset($payload['hostname'])) {
                    if ($payload['hostname'] === gethostname()) {
                        $this->processContent($payload['content_id']);
                        $message->delete(); // Message processed on the correct host, delete it
                    } else {
                        Log::info("[" . gethostname() . "] - Message received on a different host. Re-queuing or ignoring.");
                        // You can re-queue the message here if needed
                        $this->queue->push('build_podcast', $payload);
                        // Or you can simply ignore the message
                    }
                }
            } else {
                Log::info("No message found, sleeping");
                // Sleep for 30 seconds before checking the queue again
                sleep($sleep);
            }
        }
    }

    private function processContent($content_id)
    {
        $this->content = $content_id ? Content::find($content_id) : Content::where('status', 'build.podcast')
            ->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            $this->error('Content not found.');
            return 1;
        }

        try {
            $meta = json_decode($this->content->meta, true);
            $filenames = $meta['filenames'] ?? [];
            $subtitles = $meta['subtitles'] ?? [];
            $images = $meta['images'] ?? [];
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, 'podcast.built');

            $this->info("Job dispatched to publish the podcast.");
        }
    }
}
