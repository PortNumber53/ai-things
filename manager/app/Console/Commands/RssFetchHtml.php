<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Subscriptions;
use Symfony\Component\Console\Helper\ProgressBar;
use Symfony\Component\Console\Output\ConsoleOutputInterface;
use Symfony\Component\Console\Output\ConsoleOutput;
use App\Models\Collection;

class RssFetchHtml extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Rss:FetchHtml';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Process all RSS subscriptions and grab new HTML';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Get all active subscriptions
        $subscriptions = Subscriptions::where('is_active', true)->get();

        // 1. Setup main progress bar (subscription bar)
        $subscriptionBar = $this->output->createProgressBar($subscriptions->count());
        $subscriptionBar->setFormat(
            "<fg=white>[</><fg=green>▰</><fg=white>] " .
            "<fg=white>%current%/%max% [%bar%] %percent:3s%% " .
            "<fg=cyan>RSS Feed: %message%</>"
        );
        $subscriptionBar->setBarCharacter('<fg=green>▰</>');
        $subscriptionBar->setEmptyBarCharacter("<fg=white>▱</>");
        $subscriptionBar->setProgressCharacter("<fg=green>▰</>");

        // 2. Start subscription bar and create space for item bar
        $subscriptionBar->setMessage('Starting...');
        $subscriptionBar->start();
        $this->output->writeln(''); // Create line for item bar

        foreach ($subscriptions as $subscription) {
            $subscriptionBar->setMessage($subscription->feed_url);

            // Fetch RSS feed content
            $feed = simplexml_load_file($subscription->feed_url);

            // Register both required namespaces
            $feed->registerXPathNamespace('atom', 'http://www.w3.org/2005/Atom');
            $feed->registerXPathNamespace('ht', 'https://trends.google.com/trending/rss');

            // Get items (handles both formats)
            $items = isset($feed->channel) ? $feed->channel->item : $feed->item;

            // 3. Setup item progress bar
            $itemBar = new ProgressBar($this->output);
            $itemBar->setMaxSteps(count($items));
            $itemBar->setFormat(
                "<fg=white>[</><fg=green>▰</><fg=white>] " .
                "<fg=white>%current%/%max% [%bar%] %percent:3s%% " .
                "<fg=cyan>Item: %message%</>"
            );
            $itemBar->setBarCharacter('<fg=green>▰</>');
            $itemBar->setEmptyBarCharacter("<fg=white>▱</>");
            $itemBar->setProgressCharacter("<fg=green>▰</>");

            $itemBar->start();
            $itemBar->setMessage('Starting...');

            foreach ($items as $item) {
                // Get all news items for this trending item
                $newsItems = $item->children('ht', true)->news_item;

                if ($newsItems) {
                    foreach ($newsItems as $newsItem) {
                        $url = (string)$newsItem->children('ht', true)->news_item_url;

                        // Check if URL exists in Collection
                        $existingRecord = Collection::where('url', $url)->first();

                        if ($existingRecord) {
                            // Update only if html_content is empty
                            if (empty($existingRecord->html_content)) {
                                if ($htmlContent = $this->fetchHtml($url)) {
                                    $existingRecord->update([
                                        'html_content' => $htmlContent,
                                        'fetched_at' => now()
                                    ]);
                                }
                            }
                        } else {
                            // Create new record
                            if ($htmlContent = $this->fetchHtml($url)) {
                                Collection::create([
                                    'url' => $url,
                                    'title' => (string)$newsItem->children('ht', true)->news_item_title,
                                    'html_content' => $htmlContent,
                                    'language' => 'en',
                                    'fetched_at' => now()
                                ]);
                            }
                        }
                    }
                }

                // Update item bar
                $itemBar->setMessage(substr((string)$item->title, 0, 50) . "...");
                $itemBar->advance();
            }

            // Clear item bar when done with current subscription
            $itemBar->finish();
            $itemBar->clear();

            // Final subscription bar update
            // $this->output->write("\033[1A");
            $subscriptionBar->advance();
            // $this->output->writeln(''); // Maintain space for next item bar
        }

        $subscriptionBar->finish();
        $this->newLine(2);
        $this->info('RSS HTML fetch completed.');
    }

    // Add this private method to handle HTML fetching
    private function fetchHtml(string $url): ?string
    {
        try {
            // Initialize cURL session
            $ch = curl_init();

            // Set cURL options
            curl_setopt_array($ch, [
                CURLOPT_URL => $url,
                CURLOPT_RETURNTRANSFER => true,
                CURLOPT_FOLLOWLOCATION => true,
                CURLOPT_MAXREDIRS => 5,
                CURLOPT_TIMEOUT => 30,
                CURLOPT_SSL_VERIFYPEER => false,
                CURLOPT_USERAGENT => 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36'
            ]);

            // Execute the request
            $response = curl_exec($ch);

            // Check for cURL errors
            if (curl_errno($ch)) {
                $this->error("cURL error: " . curl_error($ch));
                return null;
            }

            // Close cURL session
            curl_close($ch);

            return $response;
        } catch (\Exception $e) {
            $this->error("Failed to process URL: {$url} - {$e->getMessage()}");
        }
        return null;
    }
}
