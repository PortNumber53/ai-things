<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobUploadPodcastToYoutube extends BaseJobCommand
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'job:UploadPodcastToYoutube
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';
    protected $description = 'Fix subtitles file by removing line breaks';
    protected $content;
    protected $queue;

    protected $queue_input  = 'podcast_ready';
    protected $queue_output = 'upload.tiktok';


    protected function processContent($content_id)
    {
        $this->content = $content_id ?
            Content::find($content_id) :
            Content::where('status', $this->queue_input)->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            throw new \Exception('Content not found.');
        } else {
            if ($this->content->status != $this->queue_input) {
                $this->error("content is not at the right status");
                return 1;
            }
        }

        try {
            $meta = json_decode($this->content->meta, true);
            $filenames = $meta['filenames'];


            $this->content->status = $this->queue_output;

            $this->content->save();
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the SRT file.");
        }
    }
}
