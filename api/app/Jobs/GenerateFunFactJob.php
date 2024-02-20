<?php

namespace App\Jobs;

use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Str;

class GenerateFunFactJob implements ShouldQueue
{
    use Dispatchable;
    use Queueable;

    /**
     * The prompt for generating fun facts.
     *
     * @var string
     */
    protected $prompt;

    /**
     * Create a new job instance.
     *
     * @param  string  $prompt
     * @return void
     */
    public function __construct($prompt)
    {
        $this->prompt = $prompt;
    }

    /**
     * Execute the job.
     *
     * @return void
     */
    public function handle()
    {
        // Make HTTP request to LLM/GPT service
        $response = Http::timeout(600)->post(
            'http://192.168.68.40:11434/api/generate',
            [
                'model' => 'mixtral', // notux dolphin-mistral
                'prompt' => $this->prompt,
                'stream' => false,
            ]
        );

        // Check if request was successful
        if ($response->successful()) {
            // Parse the response JSON
            $text = $response->body();

            $uuid = Str::uuid()->toString();
            $filename = public_path("/output/funfacts/{$uuid}.txt");

            $directory = dirname($filename);
            if (!is_dir($directory)) {
                mkdir($directory, 0755, true); // Create recursively with appropriate permissions
            }
            file_put_contents($filename, $text);

            // You may dispatch another job to process the saved payload if needed
            // For example, GenerateFunFactProcessorJob::dispatch($filename);
        } else {
            // Handle unsuccessful request
            // You might want to retry the job or log the failure
            // For example, $this->release(60) to retry after 60 seconds
            // Or, log the error using $response->status() or $response->body()
        }
    }
}
