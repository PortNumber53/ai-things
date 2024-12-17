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
    protected $description = 'Generates subtitles for a WAV file';

    protected $queue_input  = 'wav_generated';
    protected $queue_output = 'srt_generated';

    protected $flags_true = [
        'funfact_created',
        'wav_generated',
    ];
    protected $flags_false = [
        'srt_generated',
    ];

    protected $waiting_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
            'mp3_generated',
        ],
        false => [
            'srt_generated',
        ],
    ];

    protected $finihsed_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
            'mp3_generated',
            'srt_generated',
        ],
        false => [
            'podcast_ready',
        ],
    ];

    protected $MAX_SRT_WAITING = 100;

    protected function processContent($content_id)
    {
        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');

        // Count how many rows are processed but waiting for upload.
        $count_query = clone $base_query;
        foreach ($this->finihsed_processing_flags[true] as $flag_true) {
            $count_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->finihsed_processing_flags[false] as $flag_false) {
            $count_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }
        $count = $count_query
            ->count();
        $this->line("NEW Count query");
        $this->dq($count_query);

        // Get the rows to be processed using $waiting_processing_flags
        $work_query = clone $base_query;
        foreach ($this->waiting_processing_flags[true] as $flag_true) {
            $work_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->waiting_processing_flags[false] as $flag_false) {
            $work_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }

        if (empty($content_id)) {
            $count = $count_query
                ->count();
            if ($count >= $this->MAX_SRT_WAITING) {
                $this->info("Too many SRT waiting ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = $work_query
                    ->orderBy('id');

                // Print the generated SQL query
                // $this->line($query->toSql());

                // Execute the query and retrieve the first result
                $firstTrueRow = $query->first();
                if (!$firstTrueRow) {
                    $this->error("No content to process, sleeping 60 sec");
                    sleep(60);
                    exit(1);
                }
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }

        $this->content = Content::where('id', $content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
            throw new \Exception('Content not found.');
        }

        $current_host = config('app.hostname');
        try {
            // dump($this->content);
            $meta = json_decode($this->content->meta, true);
            // dump($meta);
            // die("STOP HERE");

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
                            $this->info("wav copied to {$this->message_hostname}");
                        }
                    }
                }

                // We run a shell script using shell_exec
                $this->info("Running shell script");
                $command = sprintf('%s %s %s', config('app.subtitle_script'), $wav_file_path, $this->content->id);
                $this->info("Running command: $command");
                $output = shell_exec($command);
                $this->info("Command output: " . $output);

                $subtitle_base_path = config('app.subtitle_folder');
                // $vtt_file_path = "$subtitle_base_path/transcription_{$this->content->id}.vtt";
                $srt_file_path = "{$subtitle_base_path}/transcription_{$this->content->id}.srt";
                $this->info("SRT path: {$srt_file_path}");

                // dump($vtt_file_path);
                dump($srt_file_path);

                // $vtt_file_contents = file_get_contents($vtt_file_path);
                $srt_file_contents = file_get_contents($srt_file_path);

                $meta['subtitles'] = [
                    // 'vtt' => $vtt_file_contents,
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
            // $job_payload = json_encode([
            //     'content_id' => $this->content->id,
            //     'hostname' => config('app.hostname'),
            // ]);
            // $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the MP3 file.");
        }
    }
}
