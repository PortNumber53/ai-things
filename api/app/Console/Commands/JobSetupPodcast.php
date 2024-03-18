<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

class JobSetupPodcast extends BaseJobCommand
{
    protected $signature = 'job:SetupPodcast
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';

    protected $description = 'Setup the podcast folder';
    protected $content;
    protected $queue;

    protected const QUEUE_INPUT  = 'generate_mp3';
    protected const QUEUE_OUTPUT = 'fix_subtitle';

    protected function processContent($content_id)
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
