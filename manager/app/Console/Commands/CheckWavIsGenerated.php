<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Log;
use App\Models\Content;
use Illuminate\Database\Eloquent\Collection;

class CheckWavIsGenerated extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Check:WavIsGenerated';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Check content in wav_generated status to make sure file exists';

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
                if (isset($meta['status']['wav_generated']) && $meta['status']['wav_generated']) {
                    if (isset($meta['wav']['filename'])) {
                        $filename = $meta['wav']['filename'];
                        $wav_file = config('app.output_folder') . "/waves/$filename";
                        if (file_exists($wav_file)) {
                            $this->info("$padded_content_id WAV file exists: " . $wav_file);
                        } else {
                            $this->error("$padded_content_id WAV file not found: " . $wav_file);
                            $reset_status = true;
                        }
                    } else {
                        $this->error("$padded_content_id WAV filename not found in meta");
                        $reset_status = true;
                    }
                }
                if ($reset_status) {
                    // We need to update wav_generated to false
                    $meta['status']['wav_generated'] = false;
                    $meta['status']['podcast_ready'] = false;
                    unset($meta['wav']);
                    $content->meta = json_encode($meta);
                    $content->save();   
                    $fixed_rows_counter++;
                }
            }
        });
        $this->info("Fixed $fixed_rows_counter rows");
    }
}
