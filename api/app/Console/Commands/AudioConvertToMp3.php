<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Contracts\Queue\Queue;
use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Log;

class AudioConvertToMp3 extends Command
{
    protected $queue;

    protected $signature = 'audio:ConvertToMp3 {content_id? : The content ID}';
    protected $description = 'Convert audio file(s) to mp3 using ffmpeg';
    protected $content;

    public function __construct(Content $content, Queue $queue)
    {
        parent::__construct();
        $this->content = $content;
        $this->queue = $queue;
    }

    public function handle()
    {
        $content_id = $this->argument('content_id');
        if (!$content_id) {
            $this->processQueueMessage();
        }
        $this->processContent($content_id);
    }

    private function processQueueMessage()
    {
        while (true) {
            $message = $this->queue->pop('generate_mp3');
            $payload = json_decode($message->getRawBody(), true);

            if (isset($payload['content_id'])) {
                $this->processContent($payload['content_id']);
                $message->delete();
            }
        }
    }

    private function processContent($content_id)
    {
        $content = $content_id ? Content::find($content_id) : Content::where('status', 'wav.generated')
            ->where('type', 'gemini.payload')->first();

        if (!$content) {
            $this->error('Content not found.');
            return 1;
        }

        $meta = json_decode($content->meta, true);
        $filenames = $meta['filenames'] ?? [];

        $convertedFiles = [];

        foreach ($filenames as $key => $filenameData) {
            $inputFile = $filenameData['filename'];
            $sentenceId = $filenameData['sentence_id'] ?? null;

            $inputFileWithPath = config('app.output_folder') . "waves/$inputFile";

            if (!File::exists($inputFileWithPath)) {
                $this->error("Input file does not exist: $inputFileWithPath");
                continue;
            }

            $outputFile = pathinfo($inputFile, PATHINFO_FILENAME) . '.mp3';
            $outputFullPath = config('app.output_folder') . "mp3/$outputFile";

            $command = "ffmpeg -y -i $inputFileWithPath -acodec libmp3lame $outputFullPath";
            exec($command, $output, $returnCode);

            if ($returnCode === 0 && File::exists($outputFullPath) && time() - File::lastModified($outputFullPath) < 60) {
                $this->info("Audio file converted successfully: $outputFullPath");
                $this->info("Audio MP3 file created properly.");
                $convertedFiles[$key] = ['filename' => $outputFile, 'sentence_id' => $sentenceId];
                $this->line("Removed $inputFileWithPath");
                unlink($inputFileWithPath);
            } else {
                $this->error("Failed to convert or create audio MP3 file.");
            }
        }

        if (!empty($convertedFiles)) {
            $content->status = 'mp3.generated';
            $meta['filenames'] = $convertedFiles;
            $content->meta = json_encode($meta);

            $content->save();
        }
    }
}
