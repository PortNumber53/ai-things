<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class ContentFindDuplicateTitles extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Content:FindDuplicateTitles';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Find the duplicate titles in the contents table';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        // Get total count of duplicate titles for progress bar
        $totalCount = \DB::table('contents')
            ->select('title', \DB::raw('COUNT(id) AS count'))
            ->groupBy('title')
            ->having(\DB::raw('COUNT(id)'), '>', 1)
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

        // Process in chunks to avoid memory issues
        \DB::table('contents')
            ->select('title')
            ->groupBy('title')
            ->having(\DB::raw('COUNT(id)'), '>', 1)
            ->orderBy('title')
            ->chunk(100, function($duplicates) use ($bar) {
                foreach ($duplicates as $duplicate) {
                    // Get count for this specific title
                    $count = \DB::table('contents')
                        ->where('title', $duplicate->title)
                        ->count();

                    $bar->setMessage("Title: " . substr($duplicate->title, 0, 50) . "... (Count: {$count})");

                    // Process the duplicate title
                    $this->processDuplicateTitle($duplicate->title);

                    $bar->advance();
                }
            });

        $bar->finish();
        $this->newLine();
        $this->info("Duplicate titles search completed.");
    }

    /**
     * Process a duplicate title and regenerate content as needed
     *
     * @param string $title The duplicate title to process
     * @return void
     */
    private function processDuplicateTitle(string $title): void
    {
        // Get all content rows with this title, ordered by view_count and comments
        $duplicates = \DB::table('contents')
            ->where('title', $title)
            ->select('id', 'title', 'meta')
            ->orderBy('id')
            ->get();

        // Initialize variables to track the "best" content
        $bestContentId = null;
        $maxViews = -1;
        $maxComments = -1;

        // Loop through duplicates to find the one with highest metrics
        foreach ($duplicates as $content) {
            $meta = json_decode($content->meta, true);
            $viewCount = $meta['view_count'] ?? 0;
            $comments = $meta['comments'] ?? 0;

            // Update best content if this one has better metrics
            if ($viewCount > $maxViews || ($viewCount == $maxViews && $comments > $maxComments)) {
                $maxViews = $viewCount;
                $maxComments = $comments;
                $bestContentId = $content->id;
            }
        }

        // Log the best content ID
        \Log::info("Best content ID for title '{$title}': {$bestContentId}");

        // Process each duplicate except the best one
        foreach ($duplicates as $content) {
            if ($content->id !== $bestContentId) {
                $this->handleDuplicateContent($content);
            }
        }
    }

    /**
     * Handle processing of a single duplicate content row
     *
     * @param object $content The content row to process
     * @return void
     */
    private function handleDuplicateContent(object $content): void
    {
        // Step 1: Get the full Content model instance
        $contentModel = Content::find($content->id);
        if (!$contentModel) {
            \Log::error("Content not found for ID: {$content->id}");
            return;
        }

        // Step 2: Prepare archive data
        $archiveEntry = [
            'data' => [
                'title' => $contentModel->title,
                'status' => $contentModel->status,
                'type' => $contentModel->type,
                'sentences' => $contentModel->sentences,
                'count' => $contentModel->count,
                'meta' => $contentModel->meta,
                'created_at' => $contentModel->created_at,
                'updated_at' => $contentModel->updated_at,
            ]
        ];

        // Step 3: Get existing archives or initialize empty array
        $archives = json_decode($contentModel->archive ?? '[]', true);

        // Step 4: Check if this exact data is already archived
        foreach ($archives as $archive) {
            if (json_encode($archive['data']) === json_encode($archiveEntry['data'])) {
                \Log::warning("Content already archived for ID: {$content->id}");
                return;
            }
        }

        // Step 5: Add new archive entry using timestamp as key
        $timestamp = now()->format('YmdHis');
        $archives[$timestamp] = $archiveEntry;

        // Step 6: Update the content with new archive
        $contentModel->archive = json_encode($archives);
        $contentModel->save();

        // Step 7: Call Artisan command to regenerate content
        \Log::info("Regenerating content for ID: {$content->id}");
        \Artisan::call('Gemini:GenerateFunFact', [
            'content_id' => $content->id
        ]);
    }
}
