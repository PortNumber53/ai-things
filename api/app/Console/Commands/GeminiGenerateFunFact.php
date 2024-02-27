<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use App\Models\Content;
use Illuminate\Support\Facades\Hash;

class GeminiGenerateFunFact extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Gemini:GenerateFunFact';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Generate JSON payload content about a random fun fact';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $apiKey = config('gemini.api_key'); // Fetch your API key from configuration

        // API Endpoint
        $url = 'https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent';

        // Request data
        $requestData = [
            'contents' => [
                [
                    'parts' => [
                        [
                            'text' => trim(<<<PROMPT
    give me a single unique random fact about any subject
    make the explanation engaging while keeping it simple
    write about 6 to 10 paragraphs, your response must be in format structured exactly like this:
    TITLE: The title for the subject comes here
    CONTENT: (the entire content about the subject goes on the next line)
    Your entire response goes here.
    PROMPT),
                        ]
                    ]
                ]
            ]
        ];

        // Make HTTP POST request
        $response = Http::withHeaders([
            'Content-Type' => 'application/json'
        ])->post($url . '?key=' . $apiKey, $requestData, [
            'key' => $apiKey
        ]);

        // Check for errors
        if ($response->failed()) {
            $this->error('Failed to generate story: ' . $response->status());
            return 1;
        }

        $title = 'Random FunFact';
        $paragraphs = [];
        $count = 0;
        $meta = [
            'gamini_response' => $response->json(),
        ];

        Content::create([
            'title' => $title,
            'status' => 'new',
            'type' => 'gemini.payload',
            'sentences' => json_encode($paragraphs),
            'count' => $count,
            'meta' => json_encode($meta),
        ]);


        // Display response
        $this->info('body: ' . $response->body());

        return 0;
    }
}
