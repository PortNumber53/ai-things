<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Http;
use App\Models\Content;
use GuzzleHttp\Client;
use Illuminate\Support\Facades\Log;

class GeminiGenerateFunFact extends BaseJobCommand
{
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
        if (!$content_id) {
            //Create
            $this->info('Inserting 1');
            $this->content = Content::create($generated_content);
        } else {
            $filter = [
                'id' => $content_id,
            ];
            $exists = Content::where('id', $content_id)->first();
            if ($exists) {
                $this->info('Upserting:: ' . $content_id);
                $this->line('filter:: ' . json_encode($filter));
                $new_content = Content::updateOrCreate($filter, $generated_content);
                Log::debug( $new_content );
            } else {
                $generated_content['id'] = $content_id;
                $this->info('Inserting:: ' . $content_id);
                // $this->title = $generated_content['title'];
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

    private function generateFunFact()
    {
        $this->job_is_processing = true;
        $apiKey = config('gemini.api_key');

        // API Endpoint
        $url = 'https://ollama.portnumber53.com/api/generate';

        // Request data
        $requestData = [
            'model' => 'llama3.2',
            'stream' => false,
            'options' => [
                'temperature' => 1,
            ],
            // 'prompt' => 'Why is the sky blue?',
            // 'stream' => 'false',
            'prompt' => trim(<<<PROMPT
Write a single unique random fact of any topic that you can think of about apple pies, 6 to 10 paragraphs about is enough,
make the explanation engaging while keeping it simple. Your response must be formatted exactly like the following example:
Here's a sample, to show the format you must use:
TITLE: The title for the subject comes here
CONTENT: The content about the fun fact goes here.
PROMPT),
        ];

        // Make HTTP POST request using GuzzleClient
        try {
            $response = $this->makeRequest($url, $requestData, $apiKey);
        } catch (\Exception $e) {
            Log::error($e->getLine());
            Log::error($e->getMessage());
            exit(1);
        }

        // Check for errors
        if ($response->getStatusCode() !== 200) {
            $statusCode = $response->getStatusCode();
            $this->error('Failed to generate fun fact. Status: ' . $statusCode);
            switch ($statusCode) {
                case 429:
                case 503:
                    sleep(5);
                    break;
                default:
                    sleep(1);
            }
            return 1;
        }

        $title = '';
        $paragraphs = [];
        $count = 0; // Counter for total entries

        // Define spacer lengths for different punctuation marks
        $punctuationSpacers = [
            '.' => 3, // Period
            '!' => 3, // Exclamation mark
            '?' => 3, // Question mark
            ';' => 2, // Semicolon (shorter spacer)
            ',' => 1, // Comma (shorter spacer)
            // Add more punctuation marks and their corresponding spacer lengths as needed
        ];

        // Extract data from the response
        $responseData = json_decode($response->getBody(), true);

        if (isset($responseData['response'])) {
            $text = str_replace("\n\n", "\n", $responseData['response']);

            $text = str_replace('***', '', $text);
            $text = str_replace('**', '', $text);
            $responseData['response'] = $text;
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
                'ollama_response' => $responseData,
            ];

            $content_create_payload = [
                'title' => $title,
                'status' => $this->queue_output,
                'type' => 'gemini.payload',
                'sentences' => json_encode($paragraphs),
                'count' => $count,
                'meta' => json_encode($meta_payload),
            ];

            $this->job_is_processing = false;
            $this->info("Title: " . $title);
            Log::debug($content_create_payload);
            // die();
            return $content_create_payload;
            // if ($content_id === false) {
            //     $content_create_payload['id'] = $content_id;
            // }
            // // Save data to database
            // if (empty($content_id)) {
            //     $this->content = Content::create($content_create_payload);

            // } else {
            //     // update

            // }
            // dump($this->content);

            // $job_payload = json_encode([
            //     'content_id' => $this->content->id,
            //     'hostname' => config('app.hostname'),
            // ]);
            // $this->queue->pushRaw($job_payload, $this->queue_output);

            // // Display success message
            // $this->info('Fun fact generated successfully.');
        } else {
            $this->error('Failed to parse response data.');
        }
        $this->job_is_processing = false;
    }

    /**
     * Make an HTTP POST request.
     *
     * @param string $url
     * @param array $data
     * @param string $apiKey
     * @return \Psr\Http\Message\ResponseInterface
     */
    private function makeRequest($url, $data, $apiKey)
    {
        $client = new Client([
            'headers' => [
                'Content-Type' => 'application/json',
                'X-API-KEY' => $apiKey // Add the API key to headers
            ]
        ]);

        $response = $client->post($url, [
            'json' => $data
        ]);

        return $response;
    }
}
