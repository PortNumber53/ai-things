<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use App\Jobs\ProcessWavFile;

class TTSPiper extends Command
{
    protected $signature = 'tts:Piper {content_id? : The content ID}';
    protected $description = 'Execute the piper shell command with dynamic parameters';
    protected $content;

    public function __construct(Content $content)
    {
        parent::__construct();
        $this->content = $content;
    }

    public function handle()
    {
        try {
            $content_id = $this->argument('content_id');
            $this->content = $content_id ? Content::find($content_id) : Content::where('status', 'new')->where('type', 'gemini.payload')->first();

            if (!$this->content) {
                throw new \Exception('Content not found.');
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
            } else {
                $this->error('Error executing piper command or output file not found or older than 1 minute.');
            }
            // Dispatch the job
            ProcessWavFile::dispatch($this->content->id, config('app.hostname'))
                ->onQueue('wav_ready');

            $this->info("Job dispatched to process the WAV file.");
        } catch (\Exception $e) {
            Log::error($e->getMessage());
            $this->error('An error occurred. Please check the logs for more details.');
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
        return trim(preg_replace('/^(TITLE:|CONTENT:)/m', '', $rawText));
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
            'echo %s | piper --debug --model %s -c %s --output_file %s',
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
        $this->content->status = 'wav.generated';
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
