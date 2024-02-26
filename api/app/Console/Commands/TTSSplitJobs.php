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
    protected $signature = 'tts:SplitJobs';

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
            'filename' => 'sample_file_rando_voice',
        ];
        $content = Content::where('status', 'new')->first();

        $voice = 'tom';

        $queueManager = app('queue');
        $queue = $queueManager->connection('rabbitmq');
        if (!empty($content['title'])) {
            $title = $content['title'];
            $jsonPayload = $jobTemplate;
            $jsonPayload['text'] = $title;
            $jsonPayload['filename'] = str_pad($title, 10, '0', STR_PAD_LEFT) . '-' .
                str_pad(0, 3, '0', STR_PAD_LEFT) . '-' . $voice . '-' . md5($title);

            $jsonPayload = json_encode($jsonPayload);
            $queue->pushRaw($jsonPayload, 'tts_wave');
            $this->line("TITLE : " . $title);
        }
        if (!empty($content['sentences'])) {
            // create job per sentence
            foreach (json_decode($content['sentences'], true) as $index => $sentence_data) {
                $text = $sentence_data['content'];
                if ($text !== '<spacer>') {
                    $jsonPayload = $jobTemplate;
                    $jsonPayload['text'] = $text;
                    $jsonPayload['filename'] = str_pad($content['id'], 10, '0', STR_PAD_LEFT) . '-' .
                        str_pad($index, 3, '0', STR_PAD_LEFT) . '-' . $voice . '-' . md5($text);


                    $jsonPayload = json_encode($jsonPayload);
                    $queue->pushRaw($jsonPayload, 'tts_wave');
                    $indexStr = str_pad($index, 10, ' ', STR_PAD_LEFT);
                    $this->line("$indexStr : " . $sentence_data['content']);
                }
            }
        }
    }
}
