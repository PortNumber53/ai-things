<?php

namespace App\Utilities;

use Illuminate\Support\Facades\Http;

class LLMProcessor
{
    public function extractSubjects(string $content): array
    {
        $prompt = "Given the following HTML content, create a list of subjects, things, events, etc. that are mentioned or implied. Format the response as a simple comma-separated list of subjects in lowercase: \n\n" . $content;

        $apiKey = config('gemini.api_key');
        $response = Http::timeout(600)
            ->connectTimeout(60)
            ->post("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key={$apiKey}", [
                'contents' => [
                    [
                        'parts' => [
                            ['text' => $prompt]
                        ]
                    ]
                ]
            ]);

        if (!$response->successful()) {
            throw new \Exception('Failed to process subjects: ' . $response->body());
        }

        $responseData = $response->json();
        $subjectText = $responseData['candidates'][0]['content']['parts'][0]['text'] ?? '';
        return array_map('trim', explode(',', trim($subjectText)));
    }
}