<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class ContentQueryCommand extends Command
{
    protected $queue;

    protected $signature = 'content:query';
    protected $description = 'Query the contents table where status is new or gemini.payload';

    public function __construct(Queue $queue)
    {
        parent::__construct();
        $this->queue = $queue;
    }

    public function handle()
    {
        // Output the initial message
        $this->info("Fetching contents with status 'new' and type 'gemini.payload'...");

        // Fetch contents in chunks of 100 records
        Content::where('status', 'new')
            ->where('type', 'gemini.payload')
            ->chunk(100, function ($contents) {
                foreach ($contents as $content) {
                    $this->line("ID: {$content->id}, Status: {$content->status}, Type: {$content->type}");

                    $job_payload = json_encode([
                        'content_id' => $content->id,
                    ]);
                    $this->queue->pushRaw($job_payload, 'generate_wav');
                }
            });

        // Output the completion message
        $this->info("Query completed.");
    }
}
