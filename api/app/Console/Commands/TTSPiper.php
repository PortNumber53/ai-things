<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class TTSPiper extends Command
{
    protected $onnx_model = '/storage/ai/models/ljspeech.onnx';
    protected $config_file = '/storage/ai/models/ljspeech.onnx.json';
    protected $voice = 'ljspeech';

    protected $content;

    protected $signature = 'tts:Piper {contentId? : The content ID}';

    protected $description = 'Execute the piper shell command with dynamic parameters';

    public function handle()
    {
        try {
            $content_id = $this->argument('contentId');
            if (empty($content_id)) {
                $this->content = Content::where('status', 'new')
                    ->where('type', 'gemini.payload')->first();
            } else {
                $this->content = Content::find($content_id);
            }
            dump($this->content);
            // die("\n\n");
            $text = $this->getText();

            $filename = $this->generateFilename($text, 1);
            $outputFile = '/output/waves/' . $filename;

            $command = 'echo ' . escapeshellarg($text) . ' | piper --debug --model ' . $this->onnx_model .
                ' -c ' . $this->config_file . ' --output_file ' . $outputFile;
            $this->line($command);
            shell_exec($command);

            if ($this->isValidOutputFile($outputFile)) {
                $this->updateContent($filename);
                $this->info("Shell command executed. Output file: $outputFile");
            } else {
                $this->error('Error executing piper command or output file not found or older than 1 minute.');
            }
        } catch (\Exception $e) {
            $this->error($e->getMessage() . $e->getLine());
        }
    }

    private function getText()
    {
        // Extract text from the meta field
        $meta = json_decode($this->content->meta, true);
        $rawText = $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'] ?? null;

        $this->line($rawText);
        if (!$rawText) {
            throw new \Exception('Text not found in the meta field.');
        }

        // Process and return text
        $text = trim($this->processText($rawText));
        $this->info($text);

        $meta = json_decode($this->content->meta, true);
        $meta['processed_text'] = $text;

        $this->content->meta = json_encode($meta);
        // dump($this->content);
        // $this->content->save();

        return $text;
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
        return $processedText;
    }

    private function generateFilename($text, $index)
    {
        // Generate filename based on content
        return sprintf("%010d-%03d-%s-%s.wav", $this->content->id, $index, $this->voice, md5($text));
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
        // dump($this->content);
        $this->content->save();
    }
}
