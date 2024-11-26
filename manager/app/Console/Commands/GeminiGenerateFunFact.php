<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Http;
use App\Models\Content;
use GuzzleHttp\Client;
use Illuminate\Support\Facades\Log;
use App\Models\Subject;
use App\Utilities\LLMProcessor;

class GeminiGenerateFunFact extends BaseJobCommand
{
    protected const PROMPT_TEMPLATE = <<<'PROMPT'
# INSTRUCTIONS
Write 10 to 15 paragraphs about a single unique random fact of any topic that you can think of about %s,
make the explanation engaging while keeping it simple. You must use the specified output format.

# SAMPLE OUTPUT FORMAT:
TITLE: The title for the subject
CONTENT: The content about the fun fact
PROMPT;

    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Gemini:GenerateFunFact
        {content_id? : The content ID}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Generate JSON payload content about a random fun fact';

    protected $queue;
    protected $content;

    protected $queue_output = 'funfact_created';

    protected $job_is_processing = false;

    protected $MAX_FUN_FACTS_WAITING = 100;

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $content_id = $this->argument('content_id');

        $count = Content::whereJsonContains("meta->status->{$this->queue_output}", true)->count();
        if (empty($content_id) && $count >= $this->MAX_FUN_FACTS_WAITING) {
            $this->info("Too many fun facts ($count) to process, sleeping for 60");
            sleep(60);
            exit();
        } else {
            // $firstTrueRow = Content::whereJsonContains('meta->status->funfact_created', true)->first();
            // $content_id = $firstTrueRow->id;
            // Now $firstTrueRow contains the first row where 'funfact_created' is true
        }



        if (empty($content_id)) {
            // Register signal handlers
            pcntl_signal(SIGINT, [$this, 'handleTerminationSignal']);
            pcntl_signal(SIGTERM, [$this, 'handleTerminationSignal']);
            pcntl_signal(SIGHUP, [$this, 'handleTerminationSignal']);
            while (true) {
                $this->generateFunFact();

                // Check for any pending signals
                pcntl_signal_dispatch();

                sleep(2);
            }
            return 0;
        }

        $generated_content = $this->generateFunFact();
        if (!$generated_content) {
            $this->error('Failed to generate content');
            return 1;
        }

        if (!$content_id) {
            $this->info('Inserting 1');
            $this->content = Content::create($generated_content);
        } else {
            $filter = [
                'id' => $content_id,
            ];
            $exists = Content::where('id', $content_id)->first();
            if ($exists) {
                $this->info('Upserting: ' . $content_id);
                $this->line('filter: ' . json_encode($filter));
                $this->line('content: ' . json_encode($generated_content));
                $new_content = Content::updateOrCreate($filter, $generated_content);
                Log::debug($new_content);
            } else {
                $generated_content['id'] = $content_id;
                $this->info('Inserting: ' . $content_id);
                $new_content = Content::create($generated_content);
                $new_content->save();
            }
        }

        // // Create content with specificied ID (should not override existing content)
        // $existing_content = Content::where('id', $content_id)->first();
        // if (!$existing_content) {
        //     $this->generateFunFact($content_id);
        // } else {
        //     dump($existing_content);
        //     $this->error("Found existing content");
        // }
    }

    public function handleTerminationSignal($signal)
    {
        // Handle termination signal
        switch ($signal) {
            case SIGINT:
                $this->info('Received SIGINT (Ctrl+C). Stopping script gracefully...');
                break;
            case SIGHUP:
                $this->info('Received SIGHUP. Stopping script gracefully...');
                break;
            case SIGKILL:
                $this->info('Received SIGKILL. Stopping script immediately...');
                exit(1); // Terminate immediately without performing any cleanup
                break;
            default:
                $this->info('Received termination signal. Stopping script gracefully...');
                break;
        }

        if ($this->job_is_processing) {
            $this->warn("Job is processing, waiting for it to finish...");
            // Loop until job finishes processing
            while ($this->job_is_processing) {
                // Sleep for a short duration to avoid tight looping
                usleep(100000); // Sleep for 100 milliseconds
            }
        }

        // Perform any cleanup operations here

        // Exit the script
        exit(0);
    }

    private function getRandomSubject()
    {
        return Subject::where('podcasts_count', '<', 1)
            ->inRandomOrder()
            ->first();
    }

    private function generateFunFact()
    {
        $this->job_is_processing = true;

        // Get random subject from database
        $subjectModel = $this->getRandomSubject();
        if (!$subjectModel) {
            $this->error('No available subjects found');
            return false;
        }
        $subject = $subjectModel->name;

        try {
            // Use LLM utility instead of direct API call
            $prompt = trim(sprintf(self::PROMPT_TEMPLATE, $subject));
            $llm = new LLMProcessor();
            $rawResponse = $llm->call_api($prompt);
            if (!$rawResponse) {
                throw new \Exception('LLM generation failed');
            }
            $text = $llm->extract_text_response($rawResponse);

        } catch (\Exception $e) {
            Log::error($e->getLine());
            Log::error($e->getMessage());
            exit(1);
        }

        // Rest of your existing response processing code...
        $title = '';
        $paragraphs = [];
        $count = 0;

        // Define spacer lengths for different punctuation marks
        $punctuationSpacers = [
            '.' => 3, // Period
            '!' => 3, // Exclamation mark
            '?' => 3, // Question mark
            ';' => 2, // Semicolon (shorter spacer)
            ',' => 1, // Comma (shorter spacer)
            // Add more punctuation marks and their corresponding spacer lengths as needed
        ];

        if (isset($rawResponse)) {
            $text = str_replace("\n\n", "\n", $text);

            $text = str_replace('***', '', $text);
            $text = str_replace('**', '', $text);

            $this->line($text);

            $responsePart = explode("\n", $text);
            $previousLineWasSpacer = false; // Flag to track if the previous line was a spacer
            foreach ($responsePart as $line) {
                if (strpos($line, 'TITLE:') === 0) {
                    $title = trim(str_replace('TITLE:', '', $line));
                } elseif (!empty($line)) {
                    $line = trim(str_replace('CONTENT:', '', $line));
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
                }
            }


            $meta_payload = [
                'status' => [
                    $this->queue_output => true,
                    'wav_generated' => false,
                    'mp3_generated' => false,
                    'podcast_ready' => false,
                    'youtube_uploaded' => false,
                    'srt_generated' => false,
                    'thumbnail_generated' => false,
                ],
                'sentences' => $paragraphs,
                'gemini_response' => $rawResponse,
            ];

            $content_create_payload = [
                'title' => $title,
                'status' => $this->queue_output,
                'type' => 'gemini.payload',
                'sentences' => json_encode($paragraphs),
                'count' => $count,
                'meta' => json_encode($meta_payload),
            ];

            // Increment the podcasts_count for the used subject
            $subjectModel->increment('podcasts_count');

            $this->job_is_processing = false;
            $this->info("Title: " . $title);
            $this->info("contents: " . json_encode($content_create_payload));
            Log::debug($content_create_payload);
            return $content_create_payload;
        } else {
            $this->error('Failed to parse response data.');
        }
        $this->job_is_processing = false;
    }
}
