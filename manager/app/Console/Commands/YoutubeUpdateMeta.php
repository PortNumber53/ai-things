<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class YoutubeUpdateMeta extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Youtube:UpdateMeta';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Crawl and update the meta data for all youtube videos';

    // Add constant for update interval
    private const UPDATE_INTERVAL_HOURS = 24;

    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Get total count for progress bar
        $totalCount = Content::whereRaw("meta->>'video_id.v1' IS NOT NULL")
            ->count();

        // Setup progress bar
        $bar = $this->output->createProgressBar($totalCount);
        $bar->setFormat(
            "<fg=white>[</><fg=green>▰</><fg=white>] " .
            "<fg=white>%current%/%max% [%bar%] %percent:3s%% " .
            "<fg=cyan>Processing: %message%</>"
        );
        $bar->setBarCharacter('<fg=green>▰</>');
        $bar->setEmptyBarCharacter("<fg=white>▱</>");
        $bar->setProgressCharacter("<fg=green>▰</>");

        $bar->setMessage('Starting...');
        $bar->start();

        // Process in chunks of 100 records
        Content::whereRaw("meta->>'video_id.v1' IS NOT NULL")
            ->chunk(100, function($contents) use ($bar) {
                foreach ($contents as $content) {
                    $bar->setMessage("Content ID: " . $content->id);

                    try {
                        $this->updateYouTubeMeta($content);
                    } catch (\Exception $e) {
                        $this->error("Error processing content ID " . $content->id . ": " . $e->getMessage());
                    }

                    $bar->advance();
                }
            });

        $bar->finish();
        $this->newLine();
        $this->info("YouTube meta update process completed.");
    }

    private function updateYouTubeMeta(Content $content)
    {
        // Get meta and check last update timestamp
        $meta = json_decode($content->meta, true);

        // Check if we have a recent update
        if (isset($meta['youtube']['meta_last_updated_at'])) {
            $lastUpdate = \Carbon\Carbon::parse($meta['youtube']['meta_last_updated_at']);
            if ($lastUpdate->diffInHours(now()) < self::UPDATE_INTERVAL_HOURS) {
                return; // Skip if updated within last 24 hours
            }
        }

        $videoId = $meta['video_id.v1'] ?? null;

        if (!$videoId) {
            throw new \Exception('Video ID not found in meta');
        }

        // Construct YouTube URL
        $url = "https://www.youtube.com/watch?v=" . $videoId;

        // Initialize cURL session
        $ch = curl_init();
        curl_setopt($ch, CURLOPT_URL, $url);
        curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
        curl_setopt($ch, CURLOPT_FOLLOWLOCATION, true);
        curl_setopt($ch, CURLOPT_USERAGENT, 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36');
        curl_setopt($ch, CURLOPT_ENCODING, 'gzip, deflate');
        curl_setopt($ch, CURLOPT_HTTPHEADER, [
            'Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language: en-US,en;q=0.5',
            'Connection: keep-alive',
            'Upgrade-Insecure-Requests: 1'
        ]);
        curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, true);
        curl_setopt($ch, CURLOPT_TIMEOUT, 30);
        curl_setopt($ch, CURLOPT_COOKIEJAR, storage_path('youtube_cookies.txt'));
        curl_setopt($ch, CURLOPT_COOKIEFILE, storage_path('youtube_cookies.txt'));

        // Execute cURL request
        $html = curl_exec($ch);

        // Get HTTP response code
        $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
        \Log::debug("YouTube response for video {$videoId}: HTTP {$httpCode}");

        if ($httpCode !== 200) {
            throw new \Exception("YouTube returned HTTP {$httpCode} for video {$videoId}");
        }

        // Debug: Check content length
        $contentLength = strlen($html);
        \Log::debug("Received {$contentLength} bytes for video {$videoId}");

        // Debug: Check if we're getting any HTML
        if (empty($html)) {
            throw new \Exception('Empty HTML response received');
        }

        // Debug: Save a sample of the HTML to inspect
        \Log::debug('First 1000 characters of HTML: ' . substr($html, 0, 1000));

        if (curl_errno($ch)) {
            throw new \Exception('Curl error: ' . curl_error($ch));
        }

        curl_close($ch);

        // Initialize meta youtube data if not exists
        $meta = json_decode($content->meta, true);
        if (!isset($meta['youtube'])) {
            $meta['youtube'] = [];
        }

        // Debug: Search for various possible patterns
        $patterns = [
            'interactionCount',
            'viewCount',
            '"videoViewCountRenderer"',
            '"viewCountText"',
            'watch-view-count'
        ];

        foreach ($patterns as $pattern) {
            if (strpos($html, $pattern) !== false) {
                \Log::debug("Found pattern: {$pattern} in response for {$videoId}");
            }
        }

        // Look for any JSON data that might contain view count
        if (preg_match('/ytInitialData = ({.+?});/', $html, $matches)) {
            \Log::debug("Found ytInitialData for {$videoId}");
            $jsonData = json_decode($matches[1], true);
            \Log::debug("JSON decode " . (json_last_error() === JSON_ERROR_NONE ? 'successful' : 'failed'));
        }

        // Update the last updated timestamp
        $meta['youtube']['meta_last_updated_at'] = now()->toIso8601String();

        // Save updated meta
        $content->meta = $meta;
        $content->save();
    }
}
