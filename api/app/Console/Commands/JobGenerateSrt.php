<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobGenerateSrt extends BaseJobCommand
{
    protected $signature = 'job:GenerateSrt
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';
    protected $description = 'Listens to queue and runs AudioConvertToMp3';
    protected $queue;
    protected $content;

    protected $queue_input  = 'generate_srt';
    protected $queue_output = 'generate_mp3';

    protected function processContent($content_id)
    {
        $this->content = $content_id ?
            Content::find($content_id) :
            Content::where('status', self: $queue_input)->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            throw new \Exception('Content not found.');
        }

        try {
            $meta = json_decode($this->content->meta, true);
            $filenames = $meta['filenames'];

            foreach ($filenames as $filename_data) {
                $wav_file_path = sprintf('%s/%s/%s', config('app.output_folder'), '/waves', $filename_data['filename']);
                // We run a shell script using shell_exec
                $this->info("Running shell script");
                $command = sprintf('%s %s %s %s', config('app.subtitle_script'), $wav_file_path, 'Transcribe', $this->content->id);
                $this->info("Running command: $command");
                $output = shell_exec($command);
                $this->info("Command output: " . $output);

                $subtitle_base_path = config('app.subtitle_folder');
                $vtt_file_path = "$subtitle_base_path/transcription_{$this->content->id}.vtt";
                $srt_file_path = "$subtitle_base_path/transcription_{$this->content->id}.srt";

                dump($vtt_file_path);
                dump($srt_file_path);

                $vtt_file_contents = file_get_contents($vtt_file_path);
                $srt_file_contents = file_get_contents($srt_file_path);

                $meta['subtitles'] = [
                    'vtt' => $vtt_file_contents,
                    'srt' => $srt_file_contents,
                ];
                $this->content->status = $this->queue_output;
                $this->content->meta = json_encode($meta);
                $this->content->save();
            }
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the MP3 file.");
        }
    }
}
