<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
// use App\Jobs\GenerateFunFact;
use Illuminate\Support\Str;

class AiGenerateFunFacts extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Ai:GenerateFunFacts';

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
        $prompt = trim(<<<PROMPT
        give me an unique random fact about a random subject of your choice.
        make the explanation engaging while keeping it simple
        write about 6 to 10 paragraphs, your response must be in JSON format structured like this:
        {"TITLE": "The title for the subject comes here",
        "CONTENT":"Each paragraph about the content shows here and keeps going as needed"}
        PROMPT);

        // Make HTTP request to LLM/GPT service
        $response = Http::timeout(600)->post(
            'http://192.168.68.40:11434/api/generate',
            [
                'model' => 'mixtral', // notux dolphin-mistral
                'prompt' => $prompt,
                'stream' => false,
            ]
        );

        // Check if request was successful
        if ($response->successful()) {
            // Parse the response JSON
            $text = $response->body();

            $uuid = Str::uuid()->toString();
            $filename = storage_path("/{$uuid}.txt");
            file_put_contents($filename, $text);
            $this->info("Payload saved to {$filename}");



            // dump($data);

            // $responseString = $data['response'];
            // dump($responseString);

            // $responseJson = json_decode($responseString, true);
            // dump($responseJson);

            // $plaintext = 'TITLE: ' . $responseJson['TITLE'] . "\n";
            // $plaintext .= 'CONTENT:' . "\n";
            // $plaintext .= $responseJson['CONTENT'] . "\n";

            // $filename = storage_path("/{$uuid}.txt");
            // file_put_contents($filename, $plaintext);
            // $this->info("Payload saved to {$filename}");

            // GenerateFunFact::dispatch($data)->onQueue('tts-fun-fact');
        } else {
            // Handle unsuccessful request
            $this->error('Failed to generate fun facts. HTTP status code: ' . $response->status());
        }
    }
}
