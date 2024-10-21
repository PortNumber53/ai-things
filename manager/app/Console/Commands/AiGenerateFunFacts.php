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
    protected $signature = 'Ai:GenerateFunFacts
            {content_id? : The content ID}
            {--sleep=30 : Sleep duration in seconds}
            {--queue : Process queue messages}
    ';

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
        $queue = $this->option('queue');
        $content_id = $this->argument('content_id');

        // while (true) {
        $timestamp = date('Y-m-d H:i:s');
        $prompt = trim(<<<PROMPT
            Write 6 to 10 paragraphs about a single unique random fact about Earth's Rotation,
            make the explanation engaging while keeping it simple.
            Your response must be in format structured exactly like this, no extra formatting required:
            TITLE: The title for the subject comes here
            CONTENT: Your entire fun fact goes here.
        PROMPT);

        $host = env('BRAIN_HOST');
        $response = Http::timeout(600)->post(
            'http://' . $host . ':11434/api/generate',
            [
                'model' => 'llama3.2', // notux dolphin-mistral tinyllama mixtral llama2
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

            if ($content_id) {
                //Update Content
                $content = Content::find($content_id);
                $content->title = $title;
                $content->sentences = json_encode($paragraphs);
                $content->count = $count;
                $content->meta = $meta;
                $result = $content->save();
            } else {
                // Save payload into database
                $result = Content::create([
                    'title' => $title,
                    'status' => 'new',
                    'type' => 'text-to-tts',
                    'sentences' => json_encode($paragraphs),
                    'count' => $count,
                    'meta' => $meta
                ]);
            }
            print_r($result);

            $this->info("{$timestamp} Fun fact generation job dispatched.");




            $text = $response->body();

            $uuid = Str::uuid()->toString();
            $filename = "funfacts/{$uuid}.txt";
        } else {
            dump($response->status());
            dump($response->body());
        }
   }
}
