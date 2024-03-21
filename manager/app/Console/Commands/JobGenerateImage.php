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
            // $imageData = Http::get('https://picsum.photos/1280/800')->body();

            // Generate filename
            $filename = sprintf("%010d.jpg", $this->content->id);
            $full_path = $this->generateImage($filename);

            // Define output path
            // $outputPath = config('app.output_folder') . "/images/$filename";

            // Save image to output folder
            // file_put_contents($outputPath, $imageData);

            // Update meta.images data point
            $meta = json_decode($this->content->meta, true);
            if (!isset($meta['images'])) {
                $meta['images'] = [];
            }
            $meta['images'][] = $full_path;
            $this->content->meta = json_encode($meta);

            $this->content->status = $this->queue_output;
            // $this->content->save();
        } catch (\Exception $e) {
            $this->error('Error occurred: ' . $e->getMessage());
            return 1;
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            // $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to upload the podcast.");
        }
    }


    protected function generateImage($filename)
    {
        $url = "http://192.168.70.87:7860";

        $data = array(
            "prompt" => "puppy dog",
            "steps" => 5
        );

        // Initialize cURL session
        $ch = curl_init();

        // Set cURL options
        curl_setopt($ch, CURLOPT_URL, $url . "/sdapi/v1/txt2img");
        curl_setopt($ch, CURLOPT_POST, 1);
        curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);

        // Execute cURL request
        $response = curl_exec($ch);

        // Check for errors
        if (curl_errno($ch)) {
            echo 'Curl error: ' . curl_error($ch);
        }

        // Close cURL session
        curl_close($ch);

        // Decode and save the image.
        $output = json_decode($response, true);
        $image_data = base64_decode($output['images'][0]);

        $full_path = config('app.output_folder') . "/images/{$filename}";
        $this->line($full_path);
        file_put_contents($full_path, $image_data);

        return $full_path;
    }
}
