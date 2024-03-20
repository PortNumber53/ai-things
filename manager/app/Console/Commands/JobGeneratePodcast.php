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
        ';
    protected $description = 'Generate PodCast but running remotion';
    protected $queue;
    protected $content;

    protected $queue_input  = 'generate_podcast';
    protected $queue_output = 'podcast_ready';

    protected function processContent($content_id)
    {
        $this->content = $content_id ?
            Content::find($content_id) :
            Content::where('status', $this->queue_input)->where('type', 'gemini.payload')->first();

        if (!$this->content) {
            throw new \Exception('Content not found.');
        } else {
            if ($this->content->status != $this->queue_input) {
                $this->error("content is not at the right status");
                return 1;
            }
        }

        try {
            $meta = json_decode($this->content->meta, true);

            $title = sprintf("%010d.jpg", $this->content->id) . " - {$this->content->title}";
            $filenames = $meta['filenames'];

            $mp3_filename = $meta['filenames'][0]['filename'];
            $duration = number_format($meta['filenames'][0]['duration'], 1);
            dump($mp3_filename);
            $source_mp3_path = config('app.output_folder') . "/mp3/{$mp3_filename}";
            $target_mp3_path = config('app.base_app_folder') . "/podcast/public/audio.mp3";
            file_put_contents($target_mp3_path, file_get_contents($source_mp3_path));

            $image_filename = $meta['images'][0];
            dump($image_filename);

            $source_image_path = config('app.output_folder') . "/images/{$image_filename}";
            $target_image_path = config('app.base_app_folder') . "/podcast/public/image.jpg";
            file_put_contents($target_image_path, file_get_contents($source_image_path));

            $srt_subtitles = $meta['subtitles']['srt'];

            // Save $srt_subtitles to temp file
            $srt_temp_file = config('app.base_app_folder') . '/podcast/public/podcast.srt';
            dump($srt_temp_file);
            file_put_contents($srt_temp_file, $srt_subtitles);

            $podcast_template_file = config('app.base_app_folder') . '/podcast/src/Root_template.tsx';
            $podcast_generated_file = config('app.base_app_folder') . '/podcast/src/Root.tsx';

            $podcast_template_contents = file_get_contents($podcast_template_file);
            $podcast_template_contents = str_replace('__REPLACE_WITH_TITLE__', $title, $podcast_template_contents);
            $podcast_template_contents = str_replace('__REPLACE_WITH_MP3__', 'audio.mp3', $podcast_template_contents);
            $podcast_template_contents = str_replace('__REPLACE_WITH_IMAGE__', 'image.jpg', $podcast_template_contents);
            $podcast_template_contents = str_replace('__REPLACE_WITH_SUBTITLES__', 'podcast.srt', $podcast_template_contents);
            $podcast_template_contents = str_replace('__DURATION__', $duration, $podcast_template_contents);


            print_r($podcast_template_contents);
            file_put_contents($podcast_generated_file, $podcast_template_contents);


            // Run remotion to generate the podcast
            $command = "cd " . config('app.base_app_folder') . "/podcast/ && npm run build";
            print_r($command);
            $output = shell_exec($command);
            print_r($output);

            $this->content->status = $this->queue_output;

            $this->content->save();
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
