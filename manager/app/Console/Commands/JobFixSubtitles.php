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
            $subtitles = $meta['subtitles'];
            $srt_contents = $subtitles['srt'];
            $vtt_contents = $subtitles['vtt'];

            $meta['subtitles']['srt'] = $this->fixSrtSubtitle($srt_contents);
            $meta['subtitles']['vtt'] = $this->fixVttSubtitle($vtt_contents);

            $this->content->status = $this->queue_output;
            $this->content->meta = json_encode($meta);
            $this->content->save();
        } catch (\Exception $e) {
            $this->error($e->getMessage());
            return 1;
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the Image file.");
        }
    }

    private function fixSrtSubtitle($srt_contents)
    {
        // Split the contents into subtitle blocks
        $subtitle_blocks = explode("\n\n", $srt_contents);

        // Iterate through each subtitle block
        foreach ($subtitle_blocks as &$block) {
            // Remove line breaks within each block
            $block = preg_replace('/\n(?![0-9]{2}:)/', ' ', $block);

            // Add a newline after the timestamp
            $block = preg_replace('/([0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{3} --> [0-9]{2}:[0-9]{2}:[0-9]{2},[0-9]{3})/', "$1\n", $block);

            // Remove leading whitespace
            $block = preg_replace('/^\s*/m', '', $block);
        }

        // Reassemble the fixed subtitle contents
        $fixed_str = implode("\n\n", $subtitle_blocks);

        return $fixed_str;
    }


    private function fixVttSubtitle($vtt_contents)
    {
        // Split the contents into subtitle blocks
        $subtitle_blocks = explode("\n\n", $vtt_contents);

        // Iterate through each subtitle block
        foreach ($subtitle_blocks as &$block) {
            // Remove leading whitespace
            $block = preg_replace('/^\s*/m', '', $block);

            // Remove line breaks within each block
            $block = preg_replace('/\n(?![0-9]{2}:[0-9]{2}\.[0-9]{3} --> [0-9]{2}:[0-9]{2}\.[0-9]{3})/', ' ', $block);

            // Add a newline after the timestamp
            $block = preg_replace('/([0-9]{2}:[0-9]{2}\.[0-9]{3} --> [0-9]{2}:[0-9]{2}\.[0-9]{3})/', "$1\n", $block);

            // Trim leading whitespace before the sentence
            $block = preg_replace('/(?<=\n)[ \t]+/', '', $block);
        }

        // Reassemble the fixed subtitle contents
        $fixed_vtt = implode("\n\n", $subtitle_blocks);

        return $fixed_vtt;
    }
}
