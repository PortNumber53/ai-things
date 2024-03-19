<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;

class TikTokPublish extends Command
{
    protected $signature = 'tiktok:publish';

    protected $description = 'Publish a video to TikTok';

    public function handle()
    {
        $accessToken = 'act.example12345Example12345Example';

        $file_upload = '/home/grimlock/ai/ai-things/podcast/out/The_fascinating_world_of_sleep.mp4';
        $videoSize = filesize($file_upload);

        $response = Http::withHeaders([
            'Authorization' => 'Bearer ' . $accessToken,
            'Content-Type' => 'application/json',
        ])->post('https://open.tiktokapis.com/v2/post/publish/inbox/video/init/', [
            'source_info' => [
                'source' => $file_upload,
                'video_size' => $videoSize,
                'chunk_size' => $videoSize,
                'total_chunk_count' => 1
            ]
        ]);

        if ($response->successful()) {
            $this->info('Video published successfully.');
        } else {
            $this->error('Failed to publish video: ' . $response->status());
        }
    }
}
