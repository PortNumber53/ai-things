<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class TTSSplitJobs extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'tts:SplitJobs {content_id?}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Queue job for TTS';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $jobTemplate = [
            'text' => 'Sample text,',
            'voice' => 'tom',
            'filename' => 'sample_file_random_voice',
            'content_id' => 0,
        ];

        $contentId = $this->argument('content_id');

        if ($contentId) {
            $content = Content::find($contentId);
        } else {
            $content = Content::where('status', 'new')->first();
        }

        if (!$content) {
            $this->error('No content found.');
            return;
        }

        $voice = 'pat';

        $queueManager = app('queue');
        $queue = $queueManager->connection('rabbitmq');
        if (!empty($content['title'])) {
            $title = $content['title'];
            $jsonPayload = $jobTemplate;
            $jsonPayload['content_id'] = $content['id'];
            $jsonPayload['text'] = $title;
            $jsonPayload['sentence_id'] = 0;
            $jsonPayload['filename'] = str_pad($content['id'], 10, '0', STR_PAD_LEFT) . '-' .
                str_pad(0, 3, '0', STR_PAD_LEFT) . '-' . $voice . '-' . md5($title) . '.wav';

            $jsonPayload = json_encode($jsonPayload);
            $queue->pushRaw($jsonPayload, 'tts_wave');
            $this->line("TITLE : " . $title);
        }
        if (!empty($content['sentences'])) {
            // create job per sentence
            foreach (json_decode($content['sentences'], true) as $index => $sentence_data) {
                $text = $sentence_data['content'];
                if (strpos($text, '<spacer ') !== 0) {
                    // Process if the text doesn't start with "<spacer "
                    $jsonPayload = $jobTemplate;
                    $jsonPayload['content_id'] = $content['id'];
                    $jsonPayload['sentence_id'] = $index; // Add the index as 'sentence_id'
                    $jsonPayload['text'] = $text;
                    $jsonPayload['filename'] = str_pad($content['id'], 10, '0', STR_PAD_LEFT) . '-' .
                        str_pad($index, 3, '0', STR_PAD_LEFT) . '-' . $voice . '-' . md5($text) . '.wav';

                    $jsonPayload = json_encode($jsonPayload);
                    dump($jsonPayload);
                    $queue->pushRaw($jsonPayload, 'tts_wave');
                    $indexStr = str_pad($index, 10, ' ', STR_PAD_LEFT);
                    $this->line("$indexStr : " . $sentence_data['content']);
                }
            }
        }
    }
}
