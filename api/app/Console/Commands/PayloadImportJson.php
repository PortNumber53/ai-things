<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Support\Facades\Hash;

class PayloadImportJson extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'payload:importJson {filename}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Imports a json as a payload for TTS';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Step 1: Get filename from command-line argument
        $filename = $this->argument('filename');

        // Step 2: Read text into memory
        // $lines = file($filename, FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
        $fileContent = file_get_contents($filename);
        $fileJson = json_decode($fileContent, true);
        if (!$fileJson) {
            die("Error converting to JSON: {$filename}");
        }
        // dump($fileJson);
        $response = $fileJson['response'];
        $responseJson = json_decode($response, true);
        $meta = json_encode([
            'original_file' => $filename,
            'hash' => Hash::make($fileContent),
            'filename' => pathinfo($filename, PATHINFO_FILENAME),
        ]);
        // dump($responseJson);

        if (!empty($responseJson['TITLE']) && !empty($responseJson['CONTENT'])) {
            $title = $responseJson['TITLE'];
            // $content = $responseJson['CONTENT'];

            $sentences = [];
            $count = 0;
            foreach ($responseJson['CONTENT'] as $line) {

                $lineSentences = array_filter(preg_split('/(?<=[.!?])\s+/', $line));
                foreach ($lineSentences as $sentence) {
                    $sentences[] = ['count' => ++$count, 'content' => trim($sentence)];
                }
                // Add spacer after each paragraph
                $sentences[] = ['count' => ++$count, 'content' => '<spacer>'];
            }
            array_pop($sentences);
            dump($title);
            echo "---------\n";
            dump($sentences);
            echo "---------\n";

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
        die("done.\n\n");
    }
}
