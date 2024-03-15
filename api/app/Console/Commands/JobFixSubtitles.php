<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobFixSubtitles extends Command
{
    protected $signature = 'job:FixSubtitles
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';
    protected $description = 'Fix subtitles file by removing line breaks';
    protected $content;
    protected $queue;

    public function __construct(Content $content, Queue $queue)
    {
        parent::__construct();
        $this->content = $content;
        $this->queue = $queue;
    }

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
            $message = $this->queue->pop('srt_ready');

            if ($message) {
                $payload = json_decode($message->getRawBody(), true);


                if (isset($payload['content_id']) && isset($payload['hostname'])) {
                    if ($payload['hostname'] === gethostname()) {
                        $this->processContent($payload['content_id']);
                        $message->delete(); // Message processed on the correct host, delete it
                    } else {
                        Log::info("[" . gethostname() . "] - Message received on a different host. Re-queuing or ignoring.");
                        // You can re-queue the message here if needed
                        $this->queue->push('str_fixed', $payload);
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
            Content::where('status', 'str_ready')->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            throw new \Exception('Content not found.');
        }
        try {
            $meta = json_decode($this->content->meta, true);
            $subtitles = $meta['subtitles'];
            $srt_contents = $subtitles['srt'];
            print_r($srt_contents);

            $meta['subtitles']['srt'] = $this->fixSubtitle($srt_contents);

            $this->content->status = 'subtitle_fixed';
            $this->content->meta = json_encode($meta);
            $this->content->save();
        } catch (\Exception $e) {
            print_r($e->getLine());
            print_r($e->getMessage());
            die("Exception\n");
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, 'subtitle_fixed');

            $this->info("Job dispatched to generate the SRT file.");
        }
    }

    private function fixSubtitle($srt_contents)
    {
        $fixed_str = '';

        $fixed_str .= $srt_contents;
        return $fixed_str;
    }
}
