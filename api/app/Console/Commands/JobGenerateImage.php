<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

class JobGenerateImage extends BaseJobCommand
{
    protected $signature = 'job:GenerateImage
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';

    protected $description = 'Generate image for podcast';
    protected $content;
    protected $queue;

    protected const QUEUE_INPUT  = 'generate_image';
    protected const QUEUE_OUTPUT = 'generate_podcast';

    protected function processContent($content_id)
    {
        $this->content = $content_id ? Content::find($content_id) : Content::where('status', self::QUEUE_INPUT)
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


            $this->content->status = self::QUEUE_OUTPUT;
            $this->content->save();
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, self::QUEUE_OUTPUT);

            $this->info("Job dispatched to upload the podcast.");
        }
    }
}
