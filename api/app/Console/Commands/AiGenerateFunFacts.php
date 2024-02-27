<?php

namespace App\Console\Commands;

use App\Jobs\GenerateFunFactJob;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Storage;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Str;
use App\Models\Content;
use Illuminate\Support\Facades\Hash;

class AiGenerateFunFacts extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Ai:GenerateFunFacts {--sleep=30 : Sleep duration in seconds}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Generate a list of fun facts';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $sleepDuration = $this->option('sleep');

        // while (true) {
        $timestamp = date('Y-m-d H:i:s');
        $prompt = trim(<<<PROMPT
            give me a single unique random fact about any subject
            make the explanation engaging while keeping it simple
            write about 6 to 10 paragraphs, your response must be in format structured exactly like this:
            TITLE: The title for the subject comes here
            CONTENT: (the entire content about the subject goes on the next line)
            Your entire response goes here.
        PROMPT);

        $host = env('BRAIN_HOST');
        $response = Http::timeout(600)->post(
            'http://' . $host . ':11434/api/generate',
            [
                'model' => 'llama2', // notux dolphin-mistral tinyllama mixtral llama2
                'keep_alive' => 300,
                'prompt' => $prompt,
                'stream' => false,
                'options' => [
                    'seed' => time(),
                    'temperature' => 1,
                ]
            ]
        );

        // Check if request was successful
        if ($response->successful()) {
            $meta = json_encode([
            ]);
            // Parse the response JSON
            // $jsonResponse = $response->json();
            // dump($jsonResponse);


            $responsePart = $response->json('response');
            dump($responsePart);

            $responsePart = explode("\n", $responsePart);
            $title = '';
            $paragraphs = [];
            $count = 0; // Counter for total entries
            $previousLineWasSpacer = false; // Flag to track if the previous line was a spacer
            foreach ($responsePart as $line) {
                if (strpos($line, 'TITLE:') === 0) {
                    $title = trim(str_replace('TITLE:', '', $line));
                } elseif (!empty($line)) {
                    $line = trim(str_replace('CONTENT:', '', $line));
                    // Break each line into sentences
                    $lineSentences = array_filter(preg_split('/(?<=[.!?])\s+/', $line));
                    foreach ($lineSentences as $sentence) {
                        $paragraphs[] = ['count' => ++$count, 'content' => trim($sentence)];
                    }
                    // Reset the flag when adding non-spacer content
                    $previousLineWasSpacer = false;
                }
                // Add spacer after each paragraph only if the previous line wasn't a spacer
                if (!$previousLineWasSpacer) {
                    $paragraphs[] = ['count' => ++$count, 'content' => '<spacer>'];
                    // Set the flag to true after adding a spacer
                    $previousLineWasSpacer = true;
                }
            }

            dump("TITLE : $title");
            echo "---------\n";
            dump($paragraphs);
            echo "---------\n";


            // die("Done...\n\n");

            // $responsePart = json_decode(json_encode($responsePart), true);

            // if (isset($responsePart['TITLE']) && isset($responsePart['CONTENT'])) {
            // $title = $responsePart['TITLE'];
            // $sentences = [];
            // $count = 0;
            // foreach ($paragraphs as $line) {
            //     $lineSentences = array_filter(preg_split('/(?<=[.!?])\s+/', $line['content']));
            //     foreach ($lineSentences as $sentence) {
            //         $sentences[] = ['count' => ++$count, 'content' => trim($sentence)];
            //     }
            //     // Add spacer after each paragraph
            //     $sentences[] = ['count' => ++$count, 'content' => '<spacer>'];
            // }
            // array_pop($sentences);
            // dump($title);
            // echo "---------\n";
            // dump($sentences);
            // echo "---------\n";

            // Save payload into database
            Content::create([
                'title' => $title,
                'status' => 'new',
                'type' => 'text-to-tts',
                'sentences' => json_encode($paragraphs),
                'count' => $count,
                'meta' => $meta
            ]);

            $this->info("{$timestamp} Fun fact generation job dispatched.");
            // }




            $text = $response->body();

            $uuid = Str::uuid()->toString();
            $filename = "funfacts/{$uuid}.txt";

            // Storage::disk('output')->put($filename, $text);

            // Log::info("Output file: $filename");

            // You may dispatch another job to process the saved payload if needed
            // For example, GenerateFunFactProcessorJob::dispatch($filename);
        } else {
            dump($response->status());
            dump($response->body());
            // Handle unsuccessful request
            // You might want to retry the job or log the failure
            // For example, $this->release(60) to retry after 60 seconds
            // Or, log the error using $response->status() or $response->body()
        }



        // Sleep for the specified duration before dispatching the next job
        //     sleep($sleepDuration);
        // }
    }
}
