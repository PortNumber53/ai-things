<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

class JobGenerateMp3 extends BaseJobCommand
{
    protected $signature = 'job:GenerateMp3
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        ';
    protected $description = 'Convert audio file(s) to mp3 using ffmpeg';
    protected $content;
    protected $queue;

    protected $queue_input  = 'wav_generated';
    protected $queue_output = 'mp3_generated';

    protected $flags_true = [
        'funfact_created',
        'wav_generated',
    ];
    protected $flags_false = [
        'mp3_generated',
    ];

    protected $waiting_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
        ],
        false => [
            'mp3_generated',
        ],
    ];

    protected $finished_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
            'mp3_generated',
        ],
        false => [
            'podcast_ready',
        ],
    ];

    protected $MAX_MP3_WAITING = 100;

    protected function processContent($content_id)
    {
        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');

        // Count how many rows are processed but waiting for upload.
        $count_query = clone $base_query;
        foreach ($this->finished_processing_flags[true] as $flag_true) {
            $count_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->finished_processing_flags[false] as $flag_false) {
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
            if ($count >= $this->MAX_MP3_WAITING) {
                $this->info("Too many MP3 waiting ($count) to process, sleeping for 60");
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


        $meta = json_decode($this->content->meta, true);
        $filenames = [$meta['wav']]; // Fake support for multiple wav files

        $convertedFiles = [];

        foreach ($filenames as $key => $filename_data) {
            dump($filename_data);
            $input_file = $filename_data['filename'];

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



            $sentenceId = $filenameData['sentence_id'] ?? null;

            $inputFileWithPath = config('app.output_folder') . "/waves/$input_file";

            if (!File::exists($inputFileWithPath)) {
                $this->error("Input file does not exist: $inputFileWithPath");
                return false;
            }

            $outputFile = pathinfo($input_file, PATHINFO_FILENAME) . '.mp3';
            $outputFullPath = config('app.output_folder') . "/mp3/$outputFile";

            $command = "ffmpeg -y -i $inputFileWithPath -acodec libmp3lame $outputFullPath";
            exec($command, $output, $returnCode);

            if ($returnCode === 0 && File::exists($outputFullPath) && time() - File::lastModified($outputFullPath) < 60) {

                $totalSeconds = 0;
                // Get mp3 duration using ffmpeg
                $command = "ffmpeg -i {$outputFullPath} 2>&1 | grep Duration";
                $output = shell_exec($command);
                $durationRegex = '/Duration: (\d+):(\d+):(\d+\.\d+)/';
                if (preg_match($durationRegex, $output, $matches)) {
                    $hours = intval($matches[1]);
                    $minutes = intval($matches[2]);
                    $seconds = floatval($matches[3]);
                    $totalSeconds = ($hours * 3600) + ($minutes * 60) + $seconds;
                    $this->info("Duration in seconds: $totalSeconds");
                }

                $this->info("Audio file converted successfully: $outputFullPath");
                $this->info("Audio MP3 file created properly.");
                $convertedFiles[$key] = [
                    'mp3' => $outputFile,
                    'sentence_id' => $sentenceId,
                    'duration' => $totalSeconds,
                    'hostname' => config('app.hostname'),
                ];
                // $this->line("Removed $inputFileWithPath");
                // unlink($inputFileWithPath);
            } else {
                $this->error("Failed to convert or create audio MP3 file.");
            }
        }

        if (!empty($convertedFiles)) {
            $this->content->status = $this->queue_output;
            $meta['mp3s'] = $convertedFiles;
            dump($meta['mp3s']);

            $meta["status"][$this->queue_output] = true;
            $this->content->meta = json_encode($meta);
            $this->content->save();

            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to fix subtitles.");
        }
    }
}
