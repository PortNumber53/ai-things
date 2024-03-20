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
        ';
    protected $description = 'Convert audio file(s) to mp3 using ffmpeg';
    protected $content;
    protected $queue;

    protected $queue_input  = 'generate_mp3';
    protected $queue_output = 'fix_subtitle';

    protected function processContent($content_id)
    {
        $this->content = $content_id ? Content::find($content_id) : Content::where('status', $this->queue_input)
            ->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            $this->error('Content not found.');
            return 1;
        }

        $meta = json_decode($this->content->meta, true);
        $filenames = $meta['filenames'] ?? [];

        $convertedFiles = [];

        foreach ($filenames as $key => $filenameData) {
            $inputFile = $filenameData['filename'];
            $sentenceId = $filenameData['sentence_id'] ?? null;

            $inputFileWithPath = config('app.output_folder') . "/waves/$inputFile";

            if (!File::exists($inputFileWithPath)) {
                $this->error("Input file does not exist: $inputFileWithPath");
                continue;
            }

            $outputFile = pathinfo($inputFile, PATHINFO_FILENAME) . '.mp3';
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
                $convertedFiles[$key] = ['filename' => $outputFile, 'sentence_id' => $sentenceId, 'duration' => $totalSeconds];
                $this->line("Removed $inputFileWithPath");
                unlink($inputFileWithPath);
            } else {
                $this->error("Failed to convert or create audio MP3 file.");
            }
        }

        if (!empty($convertedFiles)) {
            $this->content->status = $this->queue_output;
            $meta['filenames'] = $convertedFiles;
            $this->content->meta = json_encode($meta);
            $this->content->save();

            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);
        }
    }
}
