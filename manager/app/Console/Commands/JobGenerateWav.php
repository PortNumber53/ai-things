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
        ';
    protected $description = 'Execute the piper shell command with dynamic parameters';
    protected $content;

    protected $ignore_host_check = true;

    protected $queue_input  = 'generate_wav';
    protected $queue_output = 'generate_srt';

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

        $text = $this->extractTextFromMeta();

        $filename = $this->generateFilename($text, 1);
        $outputFile = config('app.output_folder') . "/waves/$filename";

        $command = $this->buildShellCommand($text, $outputFile);
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

    private function buildShellCommand($text, $outputFile)
    {
        $onnx_model = config('tts.onnx_model');
        $config_file = config('tts.config_file');

        return sprintf(
            'echo %s | piper --debug --sentence-silence 1 --model %s -c %s --output_file %s',
            escapeshellarg($text),
            $onnx_model,
            $config_file,
            $outputFile
        );
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
        $meta['filenames'][] = [
            'filename' => $filename,
            'sentence_id' => 0
        ];

        $this->content->meta = json_encode($meta);
        $this->content->save();
    }
}
