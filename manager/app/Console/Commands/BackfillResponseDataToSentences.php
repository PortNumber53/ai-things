<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class BackfillResponseDataToSentences extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Backfill:ResponseDataToSentences
        {content_id? : The content ID}
        {--log : Suppress console output and log to file instead}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Backfills processing response data texto to sentences meta datapoint';
    protected $content;

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $content_id = $this->argument('content_id');
        $logOnly = $this->option('log');

        // Helper function to handle output
        $output = function($message, $type = 'info') use ($logOnly) {
            if ($logOnly) {
                \Log::$type($message);
            } else {
                $this->$type($message);
            }
        };

        if (empty($content_id)) {
            // Get total count for progress bar
            $totalCount = Content::whereRaw("meta->>'original_text' IS NULL")
                ->orWhereRaw("meta->>'original_text' = ''")
                ->count();

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
            Content::whereRaw("meta->>'original_text' IS NULL")
                ->orWhereRaw("meta->>'original_text' = ''")
                ->chunk(100, function($contents) use ($output, $bar) {
                    foreach ($contents as $content) {
                        $this->content = $content;
                        $bar->setMessage("Content ID: " . $content->id);

                        try {
                            $originalText = $this->processContentResponse($content->id);

                            if (!empty($originalText)) {
                                $meta = json_decode($this->content->meta, true);
                                $meta['original_text'] = $originalText;
                                $this->content->meta = $meta;
                                $this->content->save();
                            }
                        } catch (\Exception $e) {
                            $output("Error processing content ID " . $content->id . ": " . $e->getMessage(), 'error');
                        }

                        $bar->advance();
                    }
                });

            $bar->finish();
            $this->newLine();
            $output("Backfill process completed.");
        } else {
            $originalText = $this->processContentResponse($content_id);

            if (!empty($originalText)) {
                $meta = json_decode($this->content->meta, true);
                $meta['original_text'] = $originalText;
                $this->content->meta = $meta;
                $this->content->save();
                $output("Original text saved.");
            }
        }
    }

    private function processContentResponse($content_id)
    {
        $this->content = Content::where('id', $content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
            throw new \Exception('Content not found.');
        }

        $meta = json_decode($this->content->meta, true);
        $this->option('log') ? \Log::debug($meta) : dump($meta);

        $ollamaResponse = $meta['ollama_response']['response'] ?? '';
        $geminiResponse = $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'] ?? '';

        if (!empty($ollamaResponse)) {
            $originalResponsePayload = $ollamaResponse;
        }
        if (!empty($geminiResponse)) {
            $originalResponsePayload = $geminiResponse;
        }
        $this->option('log') ? \Log::debug($originalResponsePayload) : dump($originalResponsePayload);

        $title = '';
        $paragraphs = [];
        $originalText = '';
        $count = 0;

        if (isset($originalResponsePayload)) {
            $text = str_replace("\n\n", "\n", $originalResponsePayload);

            $text = str_replace('***', '', $text);
            $text = str_replace('**', '', $text);
            $originalResponsePayload = $text;
            $this->option('log') ? \Log::info($text) : $this->line($text);

            $responsePart = explode("\n", $text);
            $previousLineWasSpacer = false; // Flag to track if the previous line was a spacer
            foreach ($responsePart as $line) {
                if (strpos($line, 'TITLE:') === 0) {
                    $title = trim(str_replace('TITLE:', '', $line));
                } elseif (!empty($line)) {
                    $line = trim(str_replace('CONTENT:', '', $line));
                    $originalText .= "$line\n";
                    // Break each line into sentences
                    $lineSentences = preg_split('/(?<=[.!?;,])\s+/', $line); // Use punctuation marks for splitting
                    foreach ($lineSentences as $sentence) {
                        // Determine the spacer for the punctuation mark
                        $lastChar = substr(trim($sentence), -1);
                        $spacer = isset($punctuationSpacers[$lastChar]) ? $punctuationSpacers[$lastChar] : 2;
                        if (trim($sentence) !== '') {
                            $paragraphs[] = ['count' => ++$count, 'content' => trim($sentence)];
                            // Use spacer based on punctuation
                            $paragraphs[] = ['count' => ++$count, 'content' => "<spacer $spacer>"];
                        }
                    }
                    // Reset the flag when adding non-spacer content
                    $previousLineWasSpacer = false;
                }
                // Add spacer after each paragraph only if the previous line wasn't a spacer
                if (!$previousLineWasSpacer) {
                    $paragraphs[] = ['count' => ++$count, 'content' => "<spacer 3>"]; // longer spacer for paragraphs
                    // Set the flag to true after adding a spacer
                    $previousLineWasSpacer = true;
                    $originalText .= "\n";
                }
            }
        }

        return trim($originalText);
    }

}
