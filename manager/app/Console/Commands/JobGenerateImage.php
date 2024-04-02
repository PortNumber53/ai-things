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
        {--queue : Process queue messages}
        ';

    protected $description = 'Generate image for podcast';
    protected $content;
    protected $queue;

    protected $queue_input  = 'generate_image';
    protected $queue_output = 'generate_podcast';

    protected $ignore_host_check = true;

    protected function processContent($content_id)
    {
        if (empty($content_id)) {
            $count = Content::whereJsonContains("meta->status->{$this->queue_input}", true)
                ->whereJsonDoesntContain("meta->status->{$this->queue_output}", true)
                ->count();
            if ($count >= $this->MAX_SRT_WAITING) {
                $this->info("Too many SRT ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $firstTrueRow = Content::whereJsonContains("meta->status->{$this->queue_input}", true)
                    ->where(function ($query) {
                        $query->whereJsonDoesntContain("meta->status->{$this->queue_output}", false)
                            ->orWhereNull("meta->status->{$this->queue_output}");
                    })
                    ->first();
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }

        $this->content = Content::find($content_id);
        if (empty($this->content)) {
            $this->error("Content not found.");
        }
        if (!$this->content) {
            throw new \Exception('Content not found.');
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
            $this->content->status = $this->queue_output;
            $meta["status"][$this->queue_output] = true;

            $this->content->meta = json_encode($meta);
            dump($this->content->meta);
            $this->content->save();
        } catch (\Exception $e) {
            $this->error('Error occurred: ' . $e->getMessage() . ':' . $e->getLine());
            return 1;
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the podcast.");
        }
    }


    protected function generateImage($filename)
    {
        try {
            $url = 'http://192.168.70.87:7860';
            $url = 'http://192.168.68.70:7860';

            $data = array(
                "prompt" => $this->content->title,
                "steps" => 4,
                "width" => 800,
                "height" => 600,
                // "negative_prompt" => "lowres, bad anatomy, bad hands, text, error, missing fingers, extra digit, fewer digits, cropped, worst quality, low quality, normal quality, jpeg artifacts, signature, watermark, username, blurry",
                // "enable_hr" => true,
                // "restore_faces" => true,
                // "hr_upscaler" => "Nearest",
                // "denoising_strength" => 0.7,
            );

            // Initialize cURL session
            $ch = curl_init();

            // Set cURL options
            curl_setopt($ch, CURLOPT_URL, $url . "/sdapi/v1/txt2img");
            curl_setopt($ch, CURLOPT_POST, 1);
            curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
            curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);


            curl_setopt(
                $ch,
                CURLOPT_HTTPHEADER,
                array(
                    'Content-Type: application/json',
                    'Content-Length: ' . strlen(json_encode($data))
                )
            );        // Execute cURL request
            $response = curl_exec($ch);

            // Check for errors
            if (curl_errno($ch)) {
                echo 'Curl error: ' . curl_error($ch);
            }

            // Close cURL session
            curl_close($ch);

            dump($response);
            // Decode and save the image.
            $output = json_decode($response, true);
            $image_data = base64_decode($output['images'][0]);

            $full_path = config('app.output_folder') . "/images/{$filename}";
            $this->line($full_path);
            file_put_contents($full_path, $image_data);

            $hostname = gethostname();
            if ($this->message_hostname !== $hostname) {
                // We need to scp the image to the intented host
                $this->line("Uploading image to the server...");
                $command = "scp -v {$full_path} {$this->message_hostname}:{$full_path}";
                exec($command, $output, $returnCode);
                print_r($output);
                if ($returnCode === 0) {
                    $this->info("Image moved to {$this->message_hostname}");
                }
            }

            return $filename;
        } catch (\Exception $e) {
            print_r($e->getLine());
            $this->error($e->getMessage());
        }
    }
}
