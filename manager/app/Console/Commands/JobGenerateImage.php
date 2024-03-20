<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;
use Illuminate\Support\Facades\Http;


class JobGenerateImage extends BaseJobCommand
{
    protected $signature = 'job:GenerateImage
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        ';

    protected $description = 'Generate image for podcast';
    protected $content;
    protected $queue;

    protected $queue_input  = 'generate_image';
    protected $queue_output = 'generate_podcast';

    protected function processContent($content_id)
    {
        $this->content = $content_id ? Content::find($content_id) : Content::where('status', $this->queue_input)
            ->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            $this->error('Content not found.');
            return 1;
        }

        try {
            // Fetch placeholder image from picsum.photos
            $imageData = Http::get('https://picsum.photos/1280/800')->body();

            // Generate filename
            $filename = sprintf("%010d.jpg", $this->content->id);

            // Define output path
            $outputPath = config('app.output_folder') . "/images/$filename";

            // Save image to output folder
            file_put_contents($outputPath, $imageData);

            // Update meta.images data point
            $meta = json_decode($this->content->meta, true);
            $meta['images'][] = $filename;
            $this->content->meta = json_encode($meta);

            $this->content->status = $this->queue_output;
            $this->content->save();
        } catch (\Exception $e) {
            $this->error('Error occurred: ' . $e->getMessage());
            return 1;
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to upload the podcast.");
        }
    }
}
