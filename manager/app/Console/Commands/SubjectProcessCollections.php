<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Collection;
use Illuminate\Support\Facades\Http;
use App\Models\Subject;
use App\Utilities\LLMProcessor;

class SubjectProcessCollections extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Subject:ProcessCollections';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Loops throuch all unprocessed collections and processes them';

    protected $llmProcessor;

    public function __construct(LLMProcessor $llmProcessor)
    {
        parent::__construct();
        $this->llmProcessor = $llmProcessor;
    }

    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Get total count for progress bar
        $totalCount = Collection::whereNull('processed_at')->count();

        // Setup progress bar
        $bar = $this->output->createProgressBar($totalCount);
        $bar->setFormat(
            "<fg=white>[</><fg=green>▰</><fg=white>] " .
            "<fg=white>%current%/%max% [%bar%] %percent:3s%% " .
            "<fg=cyan>Processing: %message%</>"
        );
        $bar->setBarCharacter('<fg=green>▰</>');
        $bar->setEmptyBarCharacter("<fg=white>▱</>");
        $bar->setProgressCharacter("<fg=green>▰</>");

        $bar->setMessage('Starting...');
        $bar->start();

        // Process in chunks of 100 records
        Collection::whereNull('processed_at')
            ->chunk(100, function($collections) use ($bar) {
                foreach ($collections as $collection) {
                    $bar->setMessage("Collection ID: " . $collection->id);

                    try {
                        // Get subjects from LLM
                        $subjects = $this->llmProcessor->extractSubjects($collection->html_content);
                        $this->info("Subjects: " . implode(', ', $subjects));

                        // Create subjects
                        foreach ($subjects as $subjectName) {
                            $subjectName = trim(strtolower($subjectName));
                            if (empty($subjectName)) continue;

                            Subject::firstOrCreate([
                                'subject' => $subjectName
                            ]);
                        }

                        // Update processed_at timestamp
                        $collection->processed_at = now();
                        $collection->save();
                    } catch (\Exception $e) {
                        $this->error("Error processing collection ID " . $collection->id . ": " . $e->getMessage());
                        sleep(30); // Move sleep here to handle all errors
                    }

                    $bar->advance();
                }
            });

        $bar->finish();
        $this->newLine();
        $this->info("Processing completed.");
    }
}
