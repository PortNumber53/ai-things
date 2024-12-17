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
    protected $queue_output = 'thumbnail_generated';

    protected $flags_true = [
        'funfact_created',
    ];
    protected $flags_false = [
        'thumbnail_generated',
    ];

    protected $waiting_processing_flags = [
        true => [
            'funfact_created',
        ],
        false => [
            'thumbnail_generated',
        ],
    ];

    protected $finihsed_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
            'mp3_generated',
            'srt_generated',
            'thumbnail_generated',
        ],
        false => [
            'podcast_ready',
        ],
    ];

    protected $ignore_host_check = true;

    protected $MAX_IMG_WAITING = 100;

    protected function processContent($content_id)
    {
        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');
        // Count how many rows are processed but waiting for upload.
        $count_query = clone $base_query;
        foreach ($this->finihsed_processing_flags[true] as $flag_true) {
            $count_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->finihsed_processing_flags[false] as $flag_false) {
            $count_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }
        $count = $count_query
            ->count();
        $this->line("NEW Count query");
        $this->dq($count_query);

        // Get the rows to be processed using $waiting_processing_flags
        $work_query = clone $base_query;
        foreach ($this->waiting_processing_flags[true] as $flag_true) {
            $work_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->waiting_processing_flags[false] as $flag_false) {
            $work_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }

        if (empty($content_id)) {
            $count = $count_query
                ->count();
            if ($count >= $this->MAX_IMG_WAITING) {
                $this->info("Too many IMG ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = $work_query
                    ->orderBy('id');

                // Print the generated SQL query
                // $this->line($query->toSql());

                // Execute the query and retrieve the first result
                $firstTrueRow = $query->first();

                $this->dq($query);
                dump($firstTrueRow);
                if (!$firstTrueRow) {
                    $this->error("No content to process, sleeping 60 sec");
                    sleep(60);
                    exit(1);
                }
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }
        $this->content = Content::where('id', $content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
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
            // if (!isset($meta['imag'])) {
            //     $meta['images'] = [];
            // }
            $meta['thumbnail'] = [
                'filename' => $full_path,
                'hostname' => config('app.hostname'),
            ];
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
            // $url = 'http://192.168.70.73:7860';

            $data = array(
                "prompt" => $this->content->title,
                "steps" => 32,
                "width" => 800,
                "height" => 600,
                "negative_prompt" => "lowres, bad anatomy, bad hands, text, error, missing fingers, extra digit, fewer digits, cropped, worst quality, low quality, normal quality, jpeg artifacts, signature, watermark, username, blurry",
                "enable_hr" => true,
                "restore_faces" => true,
                "hr_upscaler" => "Nearest",
                "denoising_strength" => 0.7,
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
            print_r($output);
            die("\n\n");
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
