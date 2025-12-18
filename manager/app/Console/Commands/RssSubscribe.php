<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\DB;

class RssSubscribe extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Rss:Subscribe {url : The URL of the RSS feed}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Subscribe to an RSS feed';

    /**
     * Execute the console command.
     *
     * @return int
     */
    public function handle()
    {
        $url = $this->argument('url');

        // Validate URL
        if (!filter_var($url, FILTER_VALIDATE_URL)) {
            $this->error('Invalid URL provided');
            return 1;
        }

        try {
            // Attempt to fetch the RSS feed to validate it
            $response = Http::get($url);

            // Parse the XML content
            $xml = simplexml_load_string($response->body());

            // Extract feed information
            $title = (string) ($xml->channel->title ?? null);
            $description = (string) ($xml->channel->description ?? null);
            $siteUrl = (string) ($xml->channel->link ?? null);

            // Store in subscriptions table
            DB::table('subscriptions')->insert([
                'feed_url' => $url,
                'title' => $title,
                'description' => $description,
                'site_url' => $siteUrl,
                'last_fetched_at' => now(),
                'is_active' => true,
                'created_at' => now(),
                'updated_at' => now(),
            ]);

            $this->info("Successfully subscribed to RSS feed: {$url}");
            return 0;
        } catch (\Exception $e) {
            $this->error("Failed to subscribe to RSS feed: {$e->getMessage()}");
            return 1;
        }
    }
}
