<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

class AudioConvertToMp3 extends Command
{
    protected $signature = 'audio:ConvertToMp3 {contentId : The content ID}';
    protected $description = 'Convert audio file(s) to mp3 using ffmpeg';

    public function handle()
    {
        $contentId = $this->argument('contentId');

        $content = Content::find($contentId);

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

            // Append base folder to input file path
            $inputFileWithPath = env('OUTPUT_FOLDER') . "/waves/$inputFile";

            if (!file_exists($inputFileWithPath)) {
                $this->error("Input file does not exist: $inputFileWithPath");
                die("stop\n");
            }

            $outputFile = env('OUTPUT_FOLDER') . '/waves/' . pathinfo($inputFile, PATHINFO_FILENAME) . '.mp3';

            $command = "ffmpeg -y -i $inputFileWithPath -acodec libmp3lame $outputFile";
            exec($command, $output, $returnCode);

            if ($returnCode === 0) {
                $this->info("Audio file converted successfully: $outputFile");
                if (file_exists($outputFile) && time() - filemtime($outputFile) < 60) {
                    $this->info("Audio MP3 file created properly.");
                    $convertedFiles[$key] = ['filename' => $outputFile, 'sentence_id' => $sentenceId];
                } else {
                    $this->error("Failed to create audio MP3 file properly.");
                }
            } else {
                $this->error("Failed to convert audio file.");
            }
        }

        // Update content status and filepaths after all files have been successfully processed
        if (!empty($convertedFiles)) {
            $content->status = 'mp3.generated';
            $meta['filenames'] = $convertedFiles;
            $content->meta = json_encode($meta);
            $content->save();
        }
    }
}
