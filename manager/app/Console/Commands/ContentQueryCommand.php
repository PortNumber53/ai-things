<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class ContentQueryCommand extends Command
{
    protected $queue;

    protected $signature = 'content:query {start?} {end?}';
    protected $description = 'Query the contents table where status is new or gemini.payload';

    public function __construct(Queue $queue)
    {
        parent::__construct();
        $this->queue = $queue;
    }

    public function handle()
    {
        // Output the initial message
        $this->info("Fetching contents with status 'funfact.created' and type 'gemini.payload'...");

        $start = $this->argument('start');
        $end = $this->argument('end');

        $query = Content::where('status', 'funfact.created')
            ->where('type', 'gemini.payload');

        if ($start !== null && $end !== null) {
            $query->whereBetween('id', [$start, $end]);
        }

        // Fetch contents in chunks of 10 records
        $query->chunk(10, function ($contents) {
            foreach ($contents as $content) {
                $this->line("ID: {$content->id}, Status: {$content->status}, Type: {$content->type}");

                $job_payload = json_encode([
                    'content_id' => $content->id,
                ]);
                $this->queue->pushRaw($job_payload, 'wav.generate');
            }
        });

        // Output the completion message
        $this->info("Query completed.");
    }
}
