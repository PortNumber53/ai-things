<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobGeneratePodcast extends BaseJobCommand
{
    protected $signature = 'job:GeneratePodcast
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';
    protected $description = 'Generate PodCast but running remotion';
    protected $queue;
    protected $content;

    protected $queue_input  = 'generate_podcast';
    protected $queue_output = 'podcast_ready';

    protected function processContent($content_id)
    {
        $this->content = $content_id ?
            Content::find($content_id) :
            Content::where('status', self::$queue_input)->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            throw new \Exception('Content not found.');
        } else {
            if ($this->content->status != self::$queue_input) {
                $this->error("content is not at the right status");
                return 1;
            }
        }

        try {
            $meta = json_decode($this->content->meta, true);
            $filenames = $meta['filenames'];


            $this->content->status = self::$queue_output;

            $this->content->save();
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, self::$queue_output);

            $this->info("Job dispatched to upload podcast file.");
        }
    }
}
