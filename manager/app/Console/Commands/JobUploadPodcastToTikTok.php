<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;

class JobUploadPodcastToTikTok extends BaseJobCommand
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'job:UploadPodcastToTikTok
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        {--info : Just show info, do not upload}
        ';
    protected $description = 'Uploads podcast to Tiktok';
    protected $content;
    protected $queue;

    protected $queue_input  = 'podcast_ready';
    protected $queue_output = 'upload.tiktok';

    protected $flags_true = [
        'funfact_created',
        'wav_generated',
        'mp3_generated',
        'srt_generated',
        'thumbnail_generated',
        'podcast_ready',
    ];
    protected $flags_false = [
        'tiktok_uploaded',
    ];

    protected $MAX_TIKTOK_WAITING = 100;

    protected $flags_finished = [
        'tiktok_uploaded',
    ];


    protected function processContent($content_id)
    {
        $info = $this->option('info');

        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');
        foreach ($this->flags_true as $flag_true) {
            $base_query->whereJsonContains('meta->status->' . $flag_true, true);
        }

        $count_query = clone ($base_query);
        foreach ($this->flags_false as $flag_false) {
            $count_query->whereJsonContains('meta->status->' . $flag_false, false);
        }
        $this->line("Count query");
        $this->dq($count_query);

        $work_query = clone ($base_query);
        foreach ($this->flags_false as $flag_false) {
            $work_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }
        $work_query->whereJsonDoesntContain("meta", "tiktok_video_id");
        $this->line("Work query");
        $this->dq($work_query);


        if (empty($content_id)) {
            // foreach ($this->flags_finished as $finished) {
            //     // $count_query->whereJsonContains('meta->status->' . $finished, false);
            //     $count_query->where('meta->status->' . $finished, '!=', false)
            //         ->orWhereNull('meta->status->' . $finished);
            // }
            $count_query->whereJsonDoesntContain("meta", "tiktok_video_id");

            // $count_query->WhereNot('meta->tiktok_video_id');
            $this->line("Count query::::");
            $this->dq($count_query);

            $count = $count_query
                ->count();
            if ($count >= $this->MAX_TIKTOK_WAITING) {
                $this->info("Too many podcasts waiting ($count) to be uploaded, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = $work_query
                    ->orderBy('id');

                // Print the generated SQL query
                // $this->line($query->toSql());

                // Execute the query and retrieve the first result
                $firstTrueRow = $query->first();
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
        // dump($this->content);


        try {
            $meta = json_decode($this->content->meta, true);

            $description = $this->extractTextFromMeta();

            // print_r($this->content);
            // Call script to upload video to TikTok
            $filename = config('app.output_folder') . sprintf("/podcast/%s", $meta['podcast']['filename']);
            $caption = sprintf("%07d", $this->content->id) . " - {$this->content->title}";

            $utilityDir = rtrim(config('app.base_app_folder'), '/') . '/utility';
            $command = sprintf(
                'cd %s && %s %s %s 2>&1',
                escapeshellarg($utilityDir),
                config('app.tiktok_upload_script'),
                escapeshellarg($filename),
                escapeshellarg($caption)
            );
            print_r($command);
            echo "\n\n";

            if ($info) {
                $this->line("Caption: {$caption}");
                $this->line("Description: {$description}");

                // Ask user for video ID and save it to meta
                $video_id = $this->ask("Enter video ID");
                $meta['tiktok_video_id'] = $video_id;
                $meta["status"][$this->queue_output] = true;
                $meta['status']['tiktok_uploaded'] = true;
                $this->content->status = $this->queue_output;
                $this->content->meta = json_encode($meta);
                $this->content->save();
                exit(0);
            }


            exec($command, $output, $returnCode);

            if ($returnCode === 0) {
                $this->line("Upload done.");
                print_r($output);
                $output = implode("\n", $output);

                $pattern = "/Video id '([^\']+)' was successfully uploaded/";

                // Match the pattern against the output
                if (preg_match($pattern, $output, $matches)) {
                    // Extracted video ID will be in $matches[1]
                    $videoId = $matches[1];
                    $this->line("Video ID: {$videoId}");

                    if (!isset($meta['podcast'])) {
                        $meta['podcast'] = [];
                    }
                    $meta['tiktok_video_id'] = $videoId;
                }

                $meta["status"][$this->queue_output] = true;
                $meta['status']['tiktok_uploaded'] = true;

                $this->content->status = $this->queue_output;
                $this->content->meta = json_encode($meta);
                $this->content->save();
            } else {
                $this->error("Upload failed (exit code {$returnCode}).");
                print_r($output);
            }
        } catch (\Exception $e) {
            print_r($e->getMessage());
            print_r($e->getLine());
            die("\n\n");
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            // $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to publish it to Tiktok.");
        }
    }


    private function extractTextFromMeta()
    {
        $meta = json_decode($this->content->meta, true);
        $rawText = $meta['ollama_response']['response'] ?? null;

        if (!$rawText) {
            $rawText = isset($meta['gemini_response']['candidates'][0]['content']['parts'][0]['text']) ? $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'] : null;
        }

        if (!$rawText) {
            throw new \Exception('Text not found in the meta field.');
        }

        return $this->processText($rawText);
    }


    private function processText($rawText)
    {
        $lines = explode("\n", $rawText);

        // Initialize a flag to indicate if we have encountered the title
        $processedText = '';

        // Loop through lines to process the text
        foreach ($lines as $line) {
            // Skip the line if it starts with "TITLE:"
            if (strpos($line, 'TITLE:') === 0) {
                continue;
            }

            // If the line contains "CONTENT:", remove the prefix and include the line
            if (strpos($line, 'CONTENT:') === 0) {
                $line = substr($line, strlen('CONTENT:'));
                $processedText .= $line . "\n";
                continue;
            }

            $processedText .= $line . "\n";
        }

        $processedText = str_replace("\n", " ", $processedText);
        return trim($processedText);
    }

}
