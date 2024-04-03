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
                $this->info("Too many WAV ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = Content::where('meta->status->' . $this->queue_input, true)
                    ->where(function ($query) {
                        $query->where('meta->status->' . $this->queue_output, '!=', true)
                            ->orWhereNull('meta->status->' . $this->queue_output);
                    })
                    ->orderBy('id');

                // Print the generated SQL query
                // $this->line($query->toSql());

                // Execute the query and retrieve the first result
                $firstTrueRow = $query->first();
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }

        $current_host = config('app.hostname');

        $this->content = Content::where('id', $content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
            throw new \Exception('Content not found.');
        }


        try {
            $meta = json_decode($this->content->meta, true);

            $filenames = [$meta['wav']]; // Fake array of wavs
            if (empty($meta["status"])) {
                $meta["status"] = [];
            }

            foreach ($filenames as $filename_data) {
                $wav_file_path = sprintf('%s/%s/%s', config('app.output_folder'), 'waves', $filename_data['filename']);

                $file_in_host = data_get($filename_data, 'hostname');
                $this->line("Host we are: {$current_host}");
                $this->line("File is in host: {$file_in_host}");
                if (empty($file_in_host)) {
                    $this->error("We don't know where the file is");
                } else {
                    if ($current_host != $file_in_host) {
                        $this->warn("We need to copy the file here");

                        $command = "rsync -ravp --progress {$file_in_host}:{$wav_file_path} {$wav_file_path}";
                        $this->line($command);
                        exec($command, $output, $returnCode);
                        print_r($output);
                        if ($returnCode === 0) {
                            $this->info("Image moved to {$this->message_hostname}");
                        }
                    }
                }

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
