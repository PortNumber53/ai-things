<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;

class JobGeneratePodcast extends BaseJobCommand
{
    protected $signature = 'job:GeneratePodcast
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        ';
    protected $description = 'Generate PodCast but running remotion';
    protected $queue;
    protected $content;

    protected $queue_input  = 'generate_podcast';
    protected $queue_output = 'podcast_ready';

    protected $flags_true = [
        'funfact_created',
        'wav_generated',
        'mp3_generated',
        'srt_generated',
        'thumbnail_generated',
    ];
    protected $flags_false = [
        'podcast_ready',
    ];

    protected $waiting_processing_flags = [
        true => [
            'funfact_created',
        ],
        false => [
            'podcast_ready',
        ],
    ];

    protected $finihsed_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
            'mp3_generated',
            'srt_generated',
            'podcast_ready',
        ],
        false => [
            'youtube_uploaded',
        ],
    ];

    protected $MAX_PODCAST_WAITING = 100;

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
            if ($count >= $this->MAX_PODCAST_WAITING) {
                $this->info("Too many Podcast waiting ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = $work_query
                    ->orderBy('id');

                // Print the generated SQL query
                $this->line($query->toSql());

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

            $title = sprintf("%07d", $this->content->id) . " - {$this->content->title}";
            $this->info("TITLE: $title");
            // $filenames = $meta['filenames'];


            $mp3_data = $meta['mp3s'][0];
            // $mp3_filename = $meta['mp3s'][0]['mp3'];
            $mp3_filename =  $mp3_data['mp3'];
            $mp3_file_path = sprintf('%s/%s/%s', config('app.output_folder'), 'mp3', $mp3_filename);

            $duration = number_format($meta['mp3s'][0]['duration'], 1);
            dump($meta['mp3s']);



            $file_in_host = data_get($mp3_data, 'hostname');
            $this->line("Host we are: {$current_host}");
            $this->line("File is in host: {$file_in_host}");
            if (empty($file_in_host)) {
                $this->error("We don't know where the file is");
            } else {
                if ($current_host != $file_in_host) {
                    $this->warn("We need to copy the mp3 file here");

                    $command = "rsync -ravp --progress {$file_in_host}:{$mp3_file_path} {$mp3_file_path}";
                    $this->line($command);
                    exec($command, $output, $returnCode);
                    print_r($output);
                    if ($returnCode === 0) {
                        $this->info("mp3 copied to here");
                    } else {
                        $this->error("Error coping mp3 file {$mp3_file_path}");
                        // Reset meta->status->mp3_generated to false/null
                        unset($meta['mp3s']);
                        $meta['status']['mp3_generated'] = false;
                        $this->content->meta = json_encode($meta);
                        $this->content->save();
                    }
                }
            }


            $thumbnail_data = $meta['thumbnail'];
            $image_filename = $thumbnail_data['filename'];
            dump($thumbnail_data);
            $img_file_path = sprintf('%s/%s/%s', config('app.output_folder'), 'images', $image_filename);
            dump($img_file_path);


            $file_in_host = data_get($thumbnail_data, 'hostname');
            $this->line("Host we are: {$current_host}");
            $this->line("File is in host: {$file_in_host}");
            if (empty($file_in_host)) {
                $this->error("We don't know where the file is");
            } else {
                if ($current_host != $file_in_host) {
                    $this->warn("We need to copy the image file here");

                    $command = "rsync -ravp --progress {$file_in_host}:{$img_file_path} {$img_file_path}";
                    $this->line($command);
                    exec($command, $output, $returnCode);
                    print_r($output);
                    if ($returnCode === 0) {
                        $this->info("img copied here");
                    } else {
                        $this->error("Error coping image file {$img_file_path}");
                    }
                }
            }

            // If we have an AI generate image let's use that
            $img_ai_file_path = sprintf('%s/%s/%s', config('app.output_folder'), 'images-ai', $image_filename);
            if (is_file($img_ai_file_path)) {
                dump($img_ai_file_path);

                $this->warn("We need to copy the AI image file {$img_ai_file_path} here");

                $command = "rsync -ravp --progress {$img_ai_file_path} {$img_file_path}";
                $this->line($command);
                exec($command, $output, $returnCode);
                print_r($output);
                if ($returnCode === 0) {
                    $this->info("img copied here");
                } else {
                    $this->error("Error coping image file {$img_ai_file_path}");
                }
            }







            $source_mp3_path = config('app.output_folder') . "/mp3/{$mp3_filename}";
            $target_mp3_path = config('app.base_app_folder') . "/podcast/public/audio.mp3";
            file_put_contents($target_mp3_path, file_get_contents($source_mp3_path));

            // $image_filename = $meta['images'][0];
            // dump($image_filename);

            $source_image_path = config('app.output_folder') . "/images/{$image_filename}";
            $target_image_path = config('app.base_app_folder') . "/podcast/public/image.jpg";
            file_put_contents($target_image_path, file_get_contents($source_image_path));





            $srt_subtitles = $meta['subtitles']['srt'];

            // Save $srt_subtitles to temp file
            $srt_temp_file = config('app.base_app_folder') . '/podcast/public/podcast.srt';
            $this->line("SRT FILE:");
            dump($srt_subtitles);
            $this->line("SRT file: {$srt_temp_file}");
            file_put_contents($srt_temp_file, $srt_subtitles);






            $podcast_template_file = config('app.base_app_folder') . '/podcast/src/Root_template.tsx';
            $podcast_generated_file = config('app.base_app_folder') . '/podcast/src/Root.tsx';

            $podcast_template_contents = file_get_contents($podcast_template_file);
            $podcast_template_contents = str_replace('__REPLACE_WITH_TITLE__', addslashes($title), $podcast_template_contents);
            $podcast_template_contents = str_replace('__REPLACE_WITH_MP3__', 'audio.mp3', $podcast_template_contents);
            $podcast_template_contents = str_replace('__REPLACE_WITH_IMAGE__', 'image.jpg', $podcast_template_contents);
            $podcast_template_contents = str_replace('__REPLACE_WITH_SUBTITLES__', 'podcast.srt', $podcast_template_contents);
            $podcast_template_contents = str_replace('__DURATION__', (int)$duration, $podcast_template_contents);


            print_r($podcast_template_contents);
            $this->line("remotion file generated.");
            file_put_contents($podcast_generated_file, $podcast_template_contents);


            // Run remotion to generate the podcast
            $command = "cd " . config('app.base_app_folder') . "/podcast/ && npm run build";
            print_r($command);
            $output = shell_exec($command);
            print_r($output);
            ////// TO-DO: Check for errors and throw exception if any.


            $podcast_filename = sprintf("%010d.mp4", $this->content->id);
            $podcast_folder = config('app.base_app_folder') . "/podcast/out/";
            if (!is_dir($podcast_folder)) {
                mkdir($podcast_folder, 0777, true);
            }
            $source_podcast_file = "{$podcast_folder}/video.mp4";
            $target_podcast_file = config('app.output_folder') . sprintf("/podcast/%s", $podcast_filename);

            $this->info("Copying podcast file: {$source_podcast_file} -> {$target_podcast_file}");
            copy($source_podcast_file, $target_podcast_file);

            $meta['podcast'] = [
                'filename' => $podcast_filename,
                'hostname' => config('app.hostname'),
            ];

            $meta["status"][$this->queue_output] = true;
            $this->content->meta = json_encode($meta);
            $this->content->status = $this->queue_output;

            dump($this->content->meta);
            $this->content->save();
        } catch (\Exception $e) {
            print_r($e->getMessage());
            print_r($e->getLine());
            die("\n\n");
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to upload podcast file.");
        }
    }
}
