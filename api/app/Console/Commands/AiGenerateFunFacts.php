<?php

namespace App\Console\Commands;

use App\Jobs\GenerateFunFactJob;
use Illuminate\Console\Command;

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
                give me an unique random fact about any subject e.g. the size of atoms or how food has flavor, as a few examples.
                make the explanation engaging while keeping it simple
                write about 6 to 10 paragraphs, your response must be in JSON format structured like this:
                {"TITLE": "The title for the subject comes here",
                "CONTENT":"Each paragraph about the content shows here and keeps going as needed"}
            PROMPT);

            // Dispatch the job
            GenerateFunFactJob::dispatch($prompt)->onQueue('text_fun_facts');

            $this->info("{$timestamp} Fun fact generation job dispatched.");

            // Sleep for the specified duration before dispatching the next job
            sleep($sleepDuration);
        }
    }
}
