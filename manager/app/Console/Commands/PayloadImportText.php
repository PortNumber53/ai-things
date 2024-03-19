<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Support\Facades\Hash;

class PayloadImportText extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'payload:importText {filename}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Imports a text as a payload for TTS';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Step 1: Get filename from command-line argument
        $filename = $this->argument('filename');

        // Step 2: Read text into memory
        $lines = file($filename, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
        $fileContent = file_get_contents($filename);

        // Step 3: Process the text and break it down into a JSON payload
        $title = '';
        $sentences = [];
        $count = 0; // Counter for total entries
        foreach ($lines as $line) {
            if (strpos($line, 'TITLE:') === 0) {
                $title = trim(str_replace('TITLE:', '', $line));
            } elseif (strpos($line, 'CONTENT:') === 0) {
                continue;
            } elseif (!empty($line)) {
                // Break each line into sentences
                $lineSentences = array_filter(preg_split('/(?<=[.!?])\s+/', $line));
                foreach ($lineSentences as $sentence) {
                    $sentences[] = ['count' => ++$count, 'content' => trim($sentence)];
                }
            }
            // Add spacer after each paragraph
            $sentences[] = ['count' => ++$count, 'content' => '<spacer>'];
        }

        // Remove the last spacer entry if the last line was empty (no more paragraphs)
        if (empty(trim(end($lines)))) {
            array_pop($sentences);
        }

        // Generate meta data
        $meta = json_encode([
            'original_file' => $filename,
            'hash' => Hash::make($fileContent),
            'filename' => pathinfo($filename, PATHINFO_FILENAME)
        ]);

        // Generate JSON payload
        $payload = json_encode(['title' => $title, 'sentences' => $sentences, 'count' => $count, 'meta' => $meta], JSON_PRETTY_PRINT);

        // Save payload into database
        Content::create([
            'title' => $title,
            'status' => 'new',
            'type' => 'text-to-tts',
            'sentences' => json_encode($sentences),
            'count' => $count,
            'meta' => $meta
        ]);

        // Display success message
        $this->info('Payload imported successfully and stored in the database.');
    }
}
