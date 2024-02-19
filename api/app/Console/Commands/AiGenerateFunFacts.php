<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use App\Jobs\GenerateFunFact;
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
        // Prompt for fun fact goes here
        $prompt = trim(<<<PROMPT
        give me an unique random fact about a random subject of your choice.
        make the explanation engaging while keeping it simple
        write about 20 words, your response must be in JSON format structured like this =>
        TITLE: The title for the subject comes here
        CONTENT:
        Each paragraph about the content shows here and keeps going as needed
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

        // $response = '{"model":"llama2","created_at":"2024-02-14T08:53:48.748946165Z","response":"\nHere\'s a fun fact:\n\nDid you know that the shortest war in history was between Britain and Zanzibar on August 27, 1896? Zanzibar surrendered after just 38 minutes!","done":true,"context":[518,25580,29962,3532,14816,29903,29958,5299,829,14816,29903,6778,13,13,7692,2114,5771,1244,518,29914,25580,29962,13,13,10605,29915,29879,263,2090,2114,29901,13,13,9260,366,1073,393,278,3273,342,1370,297,4955,471,1546,14933,322,796,4096,747,279,373,3111,29871,29906,29955,29892,29871,29896,29947,29929,29953,29973,796,4096,747,279,27503,287,1156,925,29871,29941,29947,6233,29991],"total_duration":6085538972,"load_duration":451430,"prompt_eval_duration":111567000,"eval_count":54,"eval_duration":5973150000}';


        // Check if request was successful
        if ($response->successful()) {
            // Parse the response JSON
            $data = $response->json();

            dump($data);
            echo json_encode($data);


            // $jsonResponse = $response
            // dump($jsonResponse);
            $responseJson = json_decode($response['response'], true);

            $plaintext = 'TITLE: ' . $responseJson['TITLE'] . "\n";
            $plaintext .= 'CONTENT:' . "\n";
            $plaintext .= $responseJson['CONTENT'] . "\n";
            $uuid = Str::uuid()->toString();

            $filename = storage_path("/{$uuid}.txt");
            file_put_contents($filename, $plaintext);



            GenerateFunFact::dispatch($data)->onQueue('tts-fun-fact');
        } else {
            // Handle unsuccessful request
            $this->error('Failed to generate fun facts. HTTP status code: ' . $response->status());
        }
    }
}
