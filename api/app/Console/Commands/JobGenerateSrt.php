<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobGenerateSrt extends Command
{
    protected $queue;
    protected $content;

    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'job:GenerateSrt
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Listens to queue and runs AudioConvertToMp3';


    public function __construct(Content $content, Queue $queue)
    {
        parent::__construct();
        $this->content = $content;
        $this->queue = $queue;
    }

    /**
     * Execute the console command.
     */
    public function handle()
    {
        try {
            $content_id = $this->argument('content_id');
            $sleep = $this->option('sleep');

            if (!$content_id) {
                $this->processQueueMessage($sleep);
            }
            $this->processContent($content_id);
        } catch (\Exception $e) {
            Log::error($e->getMessage());
            $this->error('An error occurred. Please check the logs for more details.');
        }
    }

    private function processQueueMessage($sleep)
    {
        while (true) {
            $message = $this->queue->pop('generate_srt');

            if ($message) {
                $payload = json_decode($message->getRawBody(), true);


                if (isset($payload['content_id']) && isset($payload['hostname'])) {
                    if ($payload['hostname'] === gethostname()) {
                        $this->processContent($payload['content_id']);
                        $message->delete(); // Message processed on the correct host, delete it
                    } else {
                        Log::info("[" . gethostname() . "] - Message received on a different host. Re-queuing or ignoring.");
                        // You can re-queue the message here if needed
                        $this->queue->push('generate_srt', $payload);
                        // Or you can simply ignore the message
                    }
                }
            }

            Log::info("No message found, sleeping");
            // Sleep for 30 seconds before checking the queue again
            sleep($sleep);
        }
    }


    private function processContent($content_id)
    {
        $this->content = $content_id ?
            Content::find($content_id) :
            Content::where('status', 'new')->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            throw new \Exception('Content not found.');
        }

        try {
            $meta = json_decode($this->content->meta, true);
            $filenames = $meta['filenames'];

            foreach ($filenames as $filename_data) {
                $wav_file_path = sprintf('%s/%s/%s', config('app.output_folder'), 'waves', $filename_data['filename']);
                // We run a shell script using shell_exec
                $this->info("Running shell script");
                $command = sprintf('%s %s %s %s', config('app.subtitle_script'), $wav_file_path, 'Transcribe', $this->content->id);
                $this->info("Running command: $command");
                $output = shell_exec($command);
                $this->info("Command output: " . $output);

                $subtitle_base_path = config('app.subtitle_folder');
                $vtt_file_path = "$subtitle_base_path/transcription_{$this->content->id}.vtt";
                $srt_file_path = "$subtitle_base_path/transcription_{$this->content->id}.srt";

                dump($vtt_file_path);
                dump($srt_file_path);

                $vtt_file_contents = file_get_contents($vtt_file_path);
                $srt_file_contents = file_get_contents($srt_file_path);

                $meta['subtitles'] = [
                    'vtt' => $vtt_file_contents,
                    'srt' => $srt_file_contents,
                ];
                $this->content->status = 'str_ready';
                $this->content->meta = json_encode($meta);
                $this->content->save();
            }
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, 'srt_ready');

            $this->info("Job dispatched to generate the SRT file.");
        }
    }
}
