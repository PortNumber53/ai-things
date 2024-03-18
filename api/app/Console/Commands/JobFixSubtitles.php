<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobFixSubtitles extends BaseJobCommand
{
    protected $signature = 'job:FixSubtitles
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';
    protected $description = 'Fix subtitles file by removing line breaks';
    protected $content;
    protected $queue;

    protected $queue_input  = 'fix_subtitle';
    protected $queue_output = 'generate_image';

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
            $subtitles = $meta['subtitles'];
            $srt_contents = $subtitles['srt'];
            print_r($srt_contents);

            $meta['subtitles']['srt'] = $this->fixSubtitle($srt_contents);

            $this->content->status = self::$queue_output;
            $this->content->meta = json_encode($meta);
            $this->content->save();
        } catch (\Exception $e) {
            print_r($e->getLine());
            print_r($e->getMessage());
            die("Exception\n");
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, self::$queue_output);

            $this->info("Job dispatched to generate the SRT file.");
        }
    }

    private function fixSubtitle($srt_contents)
    {
        $fixed_str = '';

        $fixed_str .= $srt_contents;
        return $fixed_str;
    }
}
