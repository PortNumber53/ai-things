<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use App\Models\Content;

class GenerateFunFact extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'gemini:generate-fun-fact';

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
        $apiKey = config('services.gemini.api_key');

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
        $response = $this->makeRequest($url, $requestData, $apiKey);

        // Check for errors
        if ($response->failed()) {
            $this->error('Failed to generate fun fact. Status: ' . $response->status());
            return 1;
        }

        // Extract data from the response
        $geminiResponse = $response->json();
        $title = 'Random Fun Fact';
        $paragraphs = [];
        $count = 0;

        // Save data to database
        Content::create([
            'title' => $title,
            'status' => 'new',
            'type' => 'gemini.payload',
            'sentences' => json_encode($paragraphs),
            'count' => $count,
            'meta' => json_encode(['gemini_response' => $geminiResponse]),
        ]);

        // Display success message
        $this->info('Fun fact generated successfully.');

        return 0;
    }

    /**
     * Make an HTTP POST request.
     *
     * @param string $url
     * @param array $data
     * @param string $apiKey
     * @return \Illuminate\Http\Client\Response
     */
    private function makeRequest($url, $data, $apiKey)
    {
        return Http::withHeaders([
            'Content-Type' => 'application/json'
        ])->post($url . '?key=' . $apiKey, $data);
    }
}
