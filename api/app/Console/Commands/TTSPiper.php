<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class TTSPiper extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'tts:Piper
                            {text? : The text to pass to piper}
                            {--model=/storage/ai/models/ljspeech.onnx : The model to use}
                            {--c=/storage/ai/models//ljspeech.onnx.json : The config file to use}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Execute the piper shell command with dynamic parameters';

    /**
     * Execute the console command.
     *
     * @return mixed
     */
    public function handle()
    {
        $text = $this->argument('text');

        // If text argument is not provided, fetch text from Contents table
        if (empty($text)) {
            // Fetch a row from the Contents table
            $content = Content::inRandomOrder()->first();

            if (!$content) {
                $this->error('No content found in the database.');
                return;
            }

            // Extract text from the meta field
            $meta = json_decode($content->meta, true);
            $text = '';

            if (isset($meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'])) {
                // Get the text
                $rawText = $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'];

                // Split the text into lines
                $lines = explode("\n", $rawText);

                // Initialize a flag to indicate if we have encountered the title
                $skipNextLine = false;

                // Loop through lines to process the text
                foreach ($lines as $line) {
                    // Skip the line if it starts with "TITLE:"
                    if (strpos($line, 'TITLE:') === 0) {
                        $skipNextLine = true;
                        continue;
                    }

                    // If the line contains "CONTENT:", remove the prefix and include the line
                    if (strpos($line, 'CONTENT:') === 0) {
                        $line = substr($line, strlen('CONTENT:'));
                        $text .= $line . "\n";
                        continue;
                    }

                    // Append the line to the text if we are not skipping it
                    if (!$skipNextLine) {
                        $text .= $line . "\n";
                    } else {
                        // Reset the flag if we have encountered the title
                        $skipNextLine = false;
                    }
                }

                // Trim any extra whitespace
                $text = trim($text);
            } else {
                $this->error('Text not found in the meta field.');
                return;
            }
        }

        $model = $this->option('model');
        $configFile = $this->option('c');

        // Generate filename
        $filename = $this->generateFilename($content, $text, 1);
        $outputFile = '/output/waves/' . $filename;

        $command = 'echo ' . escapeshellarg($text) . ' | piper --debug --model ' . $model . ' -c ' . $configFile . ' --output_file ' . $outputFile;

        // Execute the shell command
        shell_exec($command);

        // Check if the WAV file is created and recent enough
        if (file_exists($outputFile) && time() - filemtime($outputFile) <= 60) {
            // Update content status
            $content->status = 'wav.generated';
            $content->updated_at = date('Y-m-d H:i:s', time());

            // Store filename in meta field
            $meta = json_decode($content->meta, true);
            $meta['filenames'][] = [
                'filename' => $filename,
                'sentence_id' => 0 // Adjust this index as needed
            ];
            $content->meta = json_encode($meta);
            $content->save();

            // Output the result
            $this->info("Shell command executed. Output file: $outputFile");
        } else {
            $this->error('Error executing piper command or output file not found or older than 1 minute.');
        }
    }

    /**
     * Generate a filename based on content, text, and index.
     *
     * @param  Content $content
     * @param  string $text
     * @param  int $index
     * @return string
     */
    private function generateFilename(Content $content, $text, $index)
    {
        return sprintf(
            "%010d-%03d-%s-%s.wav",
            $content->id,
            $index,
            'jenny',
            md5($text)
        );
    }
}
