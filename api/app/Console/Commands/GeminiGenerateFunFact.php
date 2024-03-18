<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Http;
use App\Models\Content;
use GuzzleHttp\Client;

class GeminiGenerateFunFact extends BaseJobCommand
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

    protected $queue;
    protected $content;

    protected $queue_output = 'generate_wav';


    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Register signal handlers
        pcntl_signal(SIGINT, [$this, 'handleTerminationSignal']);
        pcntl_signal(SIGHUP, [$this, 'handleTerminationSignal']);

        while (true) {
            $this->generateFunFact();

            // Check for any pending signals
            pcntl_signal_dispatch();

            sleep(10);
        }
        return 0;
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
            default:
                $this->info('Received termination signal. Stopping script gracefully...');
                break;
        }

        // Perform any cleanup operations here

        // Exit the script
        exit(0);
    }

    private function generateFunFact()
    {
        $apiKey = config('gemini.api_key');

        // API Endpoint
        $url = 'https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent';

        // Request data
        $requestData = [
            'contents' => [
                [
                    'parts' => [
                        [
                            'text' => trim(<<<PROMPT
Write about a single unique random fact about any subject, make the explanation engaging while keeping it simple
write about 6 to 10 paragraphs, your response must be in format structured exactly like this, no extra formatting required:
TITLE: The title for the subject comes here
CONTENT: (the entire content about the subject goes on the next line)
Your entire response goes here.
PROMPT),
                        ]
                    ]
                ]
            ]
        ];

        // Make HTTP POST request using GuzzleClient
        $response = $this->makeRequest($url, $requestData, $apiKey);

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
        // dump($responseData);
        if (isset($responseData['candidates'][0]['content']['parts'][0]['text'])) {
            $text = str_replace("\n\n", "\n", $responseData['candidates'][0]['content']['parts'][0]['text']);

            $text = str_replace('***', '', $text);
            $text = str_replace('**', '', $text);
            $responseData['candidates'][0]['content']['parts'][0]['text'] = $text;
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

            // Save data to database
            $this->content = Content::create([
                'title' => $title,
                'status' => $this->queue_output,
                'type' => 'gemini.payload',
                'sentences' => json_encode($paragraphs),
                'count' => $count,
                'meta' => json_encode(['gemini_response' => $responseData]),
            ]);
            dump($this->content);

            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            // Display success message
            $this->info('Fun fact generated successfully.');
        } else {
            $this->error('Failed to parse response data.');
        }
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
                'Content-Type' => 'application/json'
            ]
        ]);

        $response = $client->post($url . '?key=' . $apiKey, [
            'json' => $data
        ]);

        return $response;
    }
}
