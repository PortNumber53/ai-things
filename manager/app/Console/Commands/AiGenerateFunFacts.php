<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use App\Jobs\GenerateFunFact;

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
        // $prompt = "I already know this: 'Hmm, how about this one: did you know that there is a species of jellyfish that is immortal? The Turritopsis dohrnii, also known as the \"immortal jellyfish,\" is a type of jellyfish that can transform its body into a younger state through a process called transdifferentiation. This means that it can essentially revert back to its polyp stage, which is the juvenile form of a jellyfish, and then grow back into an adult again. This process can be repeated indefinitely, making the Turritopsis dohrnii theoretically immortal!' tell me a fun fact about a random topic. just write the fun fact, no need to comment on mine. do not write any form of confirmation you understood my request";
        $prompt = 'give me an unique random fact about a random subject of your choice, make the explanation engaging while keeping it simple.';

        // Make HTTP request to LLM/GPT service
        $response = Http::post('http://192.168.68.40:11434/api/generate', [
            'model' => 'mistral',
            'prompt' => $prompt,
            'stream' => false,
        ]);



        // $response = '{"model":"llama2","created_at":"2024-02-14T08:53:48.748946165Z","response":"\nHere\'s a fun fact:\n\nDid you know that the shortest war in history was between Britain and Zanzibar on August 27, 1896? Zanzibar surrendered after just 38 minutes!","done":true,"context":[518,25580,29962,3532,14816,29903,29958,5299,829,14816,29903,6778,13,13,7692,2114,5771,1244,518,29914,25580,29962,13,13,10605,29915,29879,263,2090,2114,29901,13,13,9260,366,1073,393,278,3273,342,1370,297,4955,471,1546,14933,322,796,4096,747,279,373,3111,29871,29906,29955,29892,29871,29896,29947,29929,29953,29973,796,4096,747,279,27503,287,1156,925,29871,29941,29947,6233,29991],"total_duration":6085538972,"load_duration":451430,"prompt_eval_duration":111567000,"eval_count":54,"eval_duration":5973150000}';


        // Check if request was successful
        if ($response->successful()) {
            // Parse the response JSON
            $data = $response->json();

            echo json_encode($data);
            GenerateFunFact::dispatch($data)->onQueue('tts-fun-fact');
        } else {
            // Handle unsuccessful request
            $this->error('Failed to generate fun facts. HTTP status code: ' . $response->status());
        }
    }
}
