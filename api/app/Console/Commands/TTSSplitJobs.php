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
        dump($content);



        $queueManager = app('queue');
        $queue = $queueManager->connection('rabbitmq');
        if (!empty($content['title'])) {
            // create job for title
        }
        if (!empty($content['sentences'])) {
            // create job per sentence
            foreach (json_decode($content['sentences'], true) as $index => $sentence_data) {
                $text = $sentence_data['content'];
                if ($text !== '<spacer>') {
                    $jsonPayload = $jobTemplate;
                    $jsonPayload['text'] = $text;
                    $jsonPayload['filename'] = str_pad($content['id'], 10, '0', STR_PAD_LEFT) . '-' .
                        str_pad($index, 3, '0', STR_PAD_LEFT) . '-tom-' . md5($text);


                    $jsonPayload = json_encode($jsonPayload);
                    $queue->pushRaw($jsonPayload, 'tts_wave');
                }


                dump($sentence_data['content']);
            }
        }



        // dump($jsonPayload);

        // $queue->pushRaw($jsonPayload, 'testing_queue');
        // dump($content);
    }
}
