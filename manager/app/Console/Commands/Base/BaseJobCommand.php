<?php

namespace App\Console\Commands\Base;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

abstract class BaseJobCommand extends Command
{
    protected $queue;
    protected $content;

    protected $ignore_host_check = false;

    protected $queue_input  = 'queue_input';
    protected $queue_output = 'queue_output';

    protected $flags_true = [];
    protected $flags_false = [];

    protected $message_hostname;

    protected $job_is_processing = false;

    protected $flags_finished = [
        'podcast_ready',
    ];

    public function __construct(Content $content, Queue $queue)
    {
        parent::__construct();
        $this->content = $content;
        $this->queue = $queue;
    }

    public function handle()
    {
        try {
            $queue = $this->option('queue');

            $content_id = $this->argument('content_id');
            $sleep = $this->option('sleep');

            if ($queue) {
                $this->processQueueMessages($sleep);
            } else {
                $this->processContent($content_id);
            }
        } catch (\Exception $e) {
            Log::error($e->getFile());
            Log::error($e->getLine());
            Log::error($e->getMessage());
            $this->error('An error occurred. Please check the logs for more details.');
        }
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

    protected function processQueueMessages($sleep)
    {
        // Register signal handlers
        pcntl_signal(SIGINT, [$this, 'handleTerminationSignal']);
        pcntl_signal(SIGTERM, [$this, 'handleTerminationSignal']);
        pcntl_signal(SIGHUP, [$this, 'handleTerminationSignal']);

        $running = true;

        while ($running) {
            $this->line("Checking queue: " . $this->queue_input);
            $message = $this->queue->pop($this->queue_input);

            if ($message) {
                try {
                    $this->processMessage($message);
                } catch (\Exception $e) {
                    Log::error("Error processing message: " . $e->getMessage());
                    // Handle retries or logging based on your requirements
                }
            } else {
                $this->line("No message found, sleeping");
                sleep($sleep);
            }

            // Check if we received the termination signal
            pcntl_signal_dispatch();
        }
    }

    protected function processMessage($message)
    {
        $this->job_is_processing = true;
        $hostname = gethostname();
        $payload = json_decode($message->getRawBody(), true);

        if (!isset($payload['content_id'])) {
            Log::warning("Invalid message format: " . json_encode($payload));
            return;
        }

        $this->message_hostname = data_get($payload, 'hostname');
        if (!$this->ignore_host_check && $this->message_hostname !== $hostname) {
            $payload_hostname = $payload['hostname'];
            Log::info("[{$this->message_hostname}] - Message received on a different host [{$this->message_hostname}]. Re-queuing or ignoring.");
            // Handle re-queuing or ignoring the message based on your requirements
            return;
        }

        // $this->message_hostname = $payload['hostname'];
        $this->processContent($payload['content_id']);
        $message->delete();
        $this->job_is_processing = false;
    }

    protected function dq($query)
    {
        // print SQL query (with ? placeholders)
        // $this->line($query->toSql());

        // print SQL query parameter value array
        // print_r($query->getBindings());

        // print raw SQL query
        dump(vsprintf(str_replace(['?'], ['\'%s\''], $query->toSql()), $query->getBindings()));
    }
}
