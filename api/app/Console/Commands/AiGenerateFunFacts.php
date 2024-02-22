<?php

namespace App\Console\Commands;

use App\Jobs\GenerateFunFactJob;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\Storage;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Str;


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

        while (true) {
            $timestamp = date('Y-m-d H:i:s');
            $prompt = trim(<<<PROMPT
                give me an unique random fact about any subject
                make the explanation engaging while keeping it simple
                write about 6 to 10 paragraphs, your response must be in JSON format structured like this:
                {"TITLE": "The title for the subject comes here",
                "CONTENT":"Each paragraph about the content shows here and keeps going as needed"}
            PROMPT);

            $response = Http::timeout(600)->post(
                'http://192.168.68.40:11434/api/generate',
                [
                    'model' => 'mixtral', // notux dolphin-mistral tinyllama mixtral llama2
                    'keep_alive' => 300,
                    'prompt' => $prompt,
                    'stream' => false,
                    'options' => [
                        'seed' => time(),
                        'temperature' =>  1,
                    ]
                ]
            );

            // Check if request was successful
            if ($response->successful()) {
                // Parse the response JSON
                $text = $response->body();

                $uuid = Str::uuid()->toString();
                $filename = "funfacts/{$uuid}.txt";

                Storage::disk('output')->put($filename, $text);

                Log::info("Output file: $filename");

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


            $this->info("{$timestamp} Fun fact generation job dispatched.");

            // Sleep for the specified duration before dispatching the next job
            sleep($sleepDuration);
        }
    }
}
