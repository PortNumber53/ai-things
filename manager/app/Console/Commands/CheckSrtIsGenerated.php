<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Database\Eloquent\Collection;

class CheckSrtIsGenerated extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Check:SrtIsGenerated';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Check content in srt_generated status to make sure subtitle exists';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $fixed_rows_counter = 0;
        $current_host = config('app.hostname');

        Content::chunk(500, function (Collection $contents) use (&$fixed_rows_counter, $current_host) {
            foreach ($contents as $content) {
                $reset_status = false;

                $content_id = $content->id;
                $padded_content_id = str_pad($content_id, 10, '.', STR_PAD_LEFT);
                $meta = json_decode($content->meta, true);
                if (isset($meta['status']['srt_generated']) && $meta['status']['srt_generated']) {
                    $srt_filename = '/output/subtitles/transcription_' . $content_id . '.srt';
                    if (file_exists($srt_filename)) {
                        $this->info("$padded_content_id SRT file exists: " . $srt_filename);
                    } else {
                        $this->error("$padded_content_id SRT file not found: " . $srt_filename);
                        $reset_status = true;
                    }
                }
                if ($reset_status) {
                    // We need to reset flags, endpoints related to srt
                    $meta['status']['srt_generated'] = false;
                    $meta['status']['srt_fixed'] = false;
                    $meta['status']['podcast_ready'] = false;
                    unset($meta['subtitles']);
                    $content->meta = json_encode($meta);
                    $content->save();   
                    $fixed_rows_counter++;
                }
            }
        });
        $this->info("Fixed $fixed_rows_counter rows");
    }
}
