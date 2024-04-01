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
        {--queue : Process queue messages}
        ';
    protected $description = 'Listens to queue and runs AudioConvertToMp3';

    protected $queue_input  = 'wav_generated';
    protected $queue_output = 'srt_generated';

    protected $MAX_SRT_WAITING = 100;

    protected function processContent($content_id)
    {
        if (empty($content_id)) {
            $count = Content::whereJsonContains("meta->status->{$this->queue_input}", true)
                ->whereJsonDoesntContain("meta->status->{$this->queue_output}", true)
                ->count();
            if ($count >= $this->MAX_SRT_WAITING) {
                $this->info("Too many SRT ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $firstTrueRow = Content::whereJsonContains("meta->status->{$this->queue_input}", true)
                    ->where(function ($query) {
                        $query->whereJsonDoesntContain("meta->status->{$this->queue_output}", false)
                            ->orWhereNull("meta->status->{$this->queue_output}");
                    })
                    ->first();
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }

        $this->content = Content::find($content_id);
        if (empty($this->content)) {
            $this->error("Content not found.");
        }
        if (!$this->content) {
            throw new \Exception('Content not found.');
        }

        try {
            $meta = json_decode($this->content->meta, true);

            $filenames = $meta['wavs'];
            if (empty($meta["status"])) {
                $meta["status"] = [];
            }

            foreach ($filenames as $filename_data) {
                $wav_file_path = sprintf('%s/%s/%s', config('app.output_folder'), 'waves', $filename_data['filename']);
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
                $meta["status"][$this->queue_output] = true;

                $this->content->meta = json_encode($meta);
                $this->content->save();
            }
        } catch (\Exception $e) {
            print_r($e->getMessage());
            print_r($e->getLine());
            die("\n\n");
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
