<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;

class JobUploadPodcastToYoutube extends BaseJobCommand
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'job:UploadPodcastToYoutube
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        ';
    protected $description = 'Uploads podcast to youtube';
    protected $content;
    protected $queue;

    protected $queue_input  = 'podcast_ready';
    protected $queue_output = 'upload.tiktok';

    protected $flags_true = [
        'funfact_created',
        'wav_generated',
        'mp3_generated',
        'srt_generated',
        'srt_fixed',
        'thumbnail_generated',
        'podcast_ready',
    ];
    protected $flags_false = [
        'youtube_uploaded',
    ];

    protected $MAX_YOUTUBE_WAITING = 100;


    protected function processContent($content_id)
    {
        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');
        foreach ($this->flags_true as $flag_true) {
            $base_query->whereJsonContains('meta->status->' . $flag_true, true);
        }

        $count_query = clone ($base_query);
        foreach ($this->flags_false as $flag_false) {
            $count_query->whereJsonContains('meta->status->' . $flag_false, true);
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
        $this->line("Work query");
        $this->dq($work_query);


        if (empty($content_id)) {
            $count = $count_query
                ->count();
            if ($count >= $this->MAX_YOUTUBE_WAITING) {
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
        dump($this->content);


        try {
            $meta = json_decode($this->content->meta, true);

            $description = $this->extractTextFromMeta();

            print_r($this->content);
            // Call script to upload video to youtube
            $filename = config('app.output_folder') . sprintf("/podcast/%s", $meta['podcast']['filename']);
            $title = escapeshellarg(sprintf("%07d", $this->content->id) . " - {$this->content->title}");
            $description = escapeshellarg($description);
            $category = '27';
            $keywords = '';
            $privacy_status = 'public';

            $command = "cd " . config('app.base_app_folder') . "/auto-subtitles-generator/ && " . sprintf(
                '%s --file=%s --title=%s --description=%s --category=%s --keywords="%s" --privacyStatus=%s',
                config('app.youtube_upload'),
                $filename,
                $title,
                $description,
                $category,
                addslashes($keywords),
                $privacy_status
            );
            print_r($command);

            exec($command, $output, $returnCode);

            if ($returnCode === 0) {
                $this->line("Upload done.");
                print_r($output);
                $output = implode("", $output);

                $pattern = "/Video id '([^\']+)' was successfully uploaded/";

                // Match the pattern against the output
                if (preg_match($pattern, $output, $matches)) {
                    // Extracted video ID will be in $matches[1]
                    $videoId = $matches[1];
                    $this->line("Video ID: {$videoId}");

                    if (!isset($meta['podcast'])) {
                        $meta['podcast'] = [];
                    }
                    $meta['video_id.v1'] = $videoId;
                    $meta["status"][$this->queue_output] = true;

                    $this->content->status = $this->queue_output;
                    dump($this->content->meta);

                    // $this->content->save();
                } else {
                    $this->error("Video ID not found.");
                }
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
        $rawText = $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'] ?? null;

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
