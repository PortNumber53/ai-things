<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Queue\SerializableClosure;

class TTSSplitJobs extends Command
{
    protected $signature = 'tts:SplitJobs {content_id?} {sentence_id?}';
    protected $description = 'Queue job for TTS';

    protected $queue;

    public function __construct(Queue $queue)
    {
        parent::__construct();
        $this->queue = $queue;
    }

    public function handle()
    {
        $contentId = $this->argument('content_id');
        $sentenceId = $this->argument('sentence_id');

        if (!is_numeric($contentId)) {
            $this->error('Invalid content ID.');
            return;
        }

        $content = Content::find($contentId);

        if (!$content) {
            $this->error('No content found.');
            return;
        }

        if ($sentenceId !== null) {
            $this->allowReprocessingOfSentence($content, $sentenceId);
        }

        $this->processContent($content);
    }


    private function allowReprocessingOfSentence(Content $content, $sentenceId)
    {
        // Remove filename entry associated with the sentence ID
        $meta = json_decode($content->meta, true);
        $meta['filenames'] = array_values(array_filter($meta['filenames'], function ($filename) use ($sentenceId) {
            return $filename['sentence_id'] != $sentenceId;
        }));
        $content->meta = json_encode($meta);
        $content->save();

        // Reprocess the specific sentence here
        $this->info("Sentence $sentenceId reprocessed.");
    }

    private function processContent(Content $content)
    {
        $voice = 'jenny';

        if (!empty($content['title'])) {
            $this->createJob($content, $content['title'], 0);
        }

        if (!empty($content['sentences'])) {
            foreach (json_decode($content['sentences'], true) as $index => $sentence_data) {
                $text = $sentence_data['content'];

                if (strpos($text, '<spacer ') !== 0) {
                    $this->createJob($content, $text, $index);
                }
            }
        }
    }

    private function createJob(Content $content, $text, $index)
    {
        $jobPayload = $this->generateJobPayload($content, $text, $index);
        $this->queue->pushRaw($jobPayload, 'tts_wave');
        $this->line("$index : " . $text);
    }

    private function generateJobPayload(Content $content, $text, $index)
    {
        $jobTemplate = [
            'text' => $text,
            'voice' => 'jenny',
            'filename' => $this->generateFilename($content, $text, $index),
            'content_id' => $content->id,
            'sentence_id' => $index
        ];

        return json_encode($jobTemplate);
    }

    private function generateFilename(Content $content, $text, $index)
    {
        return sprintf(
            "%010d-%03d-%s-%s.wav",
            $content->id,
            $index,
            'jenny',
            md5($text)
        );
    }
}
