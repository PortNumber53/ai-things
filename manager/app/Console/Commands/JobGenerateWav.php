<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use App\Jobs\ProcessWavFile;

class JobGenerateWav extends BaseJobCommand
{
    protected $queue;

    protected $signature = 'job:GenerateWav
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        ';
    protected $description = 'Execute the piper shell command with dynamic parameters';
    protected $content;

    protected $ignore_host_check = true;

    protected $queue_input  = 'funfact_created';
    protected $queue_output = 'wav_generated';

    private const PRE_SILENCE = 2;
    private const POST_SILENCE = 5;

    protected $MAX_WAV_WAITING = 100;

    protected function processContent($content_id)
    {
        if (empty($content_id)) {
            $count = Content::whereJsonContains("meta->status->{$this->queue_input}", true)
                ->whereJsonDoesntContain("meta->status->{$this->queue_output}", true)
                ->count();
            if ($count >= $this->MAX_WAV_WAITING) {
                $this->info("Too many WAV ($count) to process, sleeping for 60");
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

        $this->content = Content::find($content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
        }

        if (!$this->content) {
            throw new \Exception('Content not found.');
        }

        $text = $this->extractTextFromMeta();

        $filename = $this->generateFilename($text, 1);
        $outputFile = config('app.output_folder') . "/waves/$filename";

        $command = $this->buildShellCommand($text, $filename);
        $this->line($command);
        shell_exec($command);

        if ($this->isValidOutputFile($outputFile)) {
            $this->updateContent($filename);
            $this->info("Shell command executed. Output file: $outputFile");

            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the SRT file.");
        } else {
            $this->error('Error executing piper command or output file not found or older than 1 minute.');
        }
    }

    private function extractTextFromMeta()
    {
        $meta = json_decode($this->content->meta, true);
        $rawText = $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'] ?? null;

        if (!$rawText) {
            throw new \Exception('Text not found in the meta field.');
        }

        return $this->processText($rawText);
    }

    private function processText($rawText)
    {
        $lines = explode("\n", $rawText);

        // Initialize a flag to indicate if we have encountered the title
        $processedText = '';

        // Loop through lines to process the text
        foreach ($lines as $line) {
            // Skip the line if it starts with "TITLE:"
            if (strpos($line, 'TITLE:') === 0) {
                continue;
            }

            // If the line contains "CONTENT:", remove the prefix and include the line
            if (strpos($line, 'CONTENT:') === 0) {
                $line = substr($line, strlen('CONTENT:'));
                $processedText .= $line . "\n";
                continue;
            }

            $processedText .= $line . "\n";
        }
        return trim($processedText);
    }

    private function generateFilename($text, $index)
    {
        $voice = config('tts.voice');
        return sprintf("%010d-%03d-%s-%s.wav", $this->content->id, $index, $voice, md5($text));
    }

    private function buildShellCommand($text, $filename)
    {
        $preFile = config('app.output_folder') . "/waves/pre-$filename";
        $outputFile = config('app.output_folder') . "/waves/$filename";

        $onnx_model = config('tts.onnx_model');
        $config_file = config('tts.config_file');

        // Original command to generate WAV file
        $originalCommand = sprintf(
            'echo %s | piper --debug --sentence-silence 0.7 --model %s -c %s --output_file %s',
            escapeshellarg($text),
            $onnx_model,
            $config_file,
            $preFile
        );

        // Command to add silence at the end of the generated WAV file
        $soxCommand = sprintf(
            'sox %s %s pad %d %d',
            escapeshellarg($preFile),
            escapeshellarg($outputFile),
            self::PRE_SILENCE,
            self::POST_SILENCE
        );
        $this->line($soxCommand);

        // Combine the original command and the silence addition command
        return $originalCommand . ' && ' . $soxCommand;
    }

    private function isValidOutputFile($outputFile)
    {
        return file_exists($outputFile) && time() - filemtime($outputFile) <= 60;
    }

    private function updateContent($filename)
    {
        $this->content->status = $this->queue_output;
        $this->content->updated_at = now();

        $meta = json_decode($this->content->meta, true);
        if (!isset($meta['wav'])) {
            $meta['wavs'] = [];
        }
        $meta['wavs'][] = [
            'filename' => $filename,
            'sentence_id' => 0
        ];
        if (empty($meta["status"])) {
            $meta["status"] = [];
        }

        // $meta["status"]['funfact_created'] = true;
        $meta["status"][$this->queue_output] = true;

        $this->content->meta = json_encode($meta);
        $this->content->save();
    }
}
