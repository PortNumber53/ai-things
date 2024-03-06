<?php

namespace App\Console\Commands;

use AllowDynamicProperties;
use Illuminate\Console\Command;
use App\Models\Content;
use Illuminate\Support\Facades\Artisan;

class CheckSentencesMatchCommand extends Command
{
    protected $signature = 'sentences:check {id?}';

    protected $description = 'Check if all entries in sentences have a corresponding entry in meta.filenames.';

    public function __construct()
    {
        parent::__construct();
    }

    public function handle()
    {
        $id = $this->argument('id');

        if ($id) {
            $records = Content::where('id', $id)->get();
        } else {
            $records = Content::where('type', 'gemini.payload')->get();
        }

        $modifiedCount = 0;

        foreach ($records as $record) {
            $sentences = collect(json_decode($record->sentences, true))->reject(function ($sentence) {
                return strpos($sentence['content'], '<spacer') === 0;
            });

            $meta = optional(json_decode($record->meta));
            $filenames = collect($meta->filenames ?? [])->toArray(); // Convert to array

            // Check for title filename
            $titleFilename = sprintf('%010d-%03d-', $record->id, 0);
            $titleFilenameExists = false;

            $allMatch = true;
            foreach ($sentences as $sentence) {
                $sentenceId = $sentence['count'];
                $match = collect($filenames)->contains('sentence_id', $sentenceId);
                if (!$match) {
                    $this->info("Record with ID $record->id sentence $sentenceId has no filename.");
                    $allMatch = false;
                    break;
                }
            }

            // Check for title filename
            foreach ($filenames as $filenameData) {
                if (strpos($filenameData->filename, $titleFilename) !== false) { // Accessing as object
                    // Partial match found for title filename
                    $titleFilenameExists = true;
                    break;
                }
            }

            if (!$titleFilenameExists) {
                $this->info("Record with ID $record->id has no title filename.");
                $allMatch = false;
            }

            $allMatchStr = ($allMatch === true) ? 'all match' : 'not all match';
            $this->line("Record with ID $record->id and $allMatchStr");
            if ($allMatch && $record->type !== 'gemini.wav_ready') {
                $record->type = 'gemini.wav_ready';
                $record->save();
                $modifiedCount++;
            }
        }



        // Call the tts:SplitJobs command after records are updated
        if ($modifiedCount > 0) {
            Artisan::call('tts:SplitJobs');
        }

        if ($id) {
            if ($modifiedCount > 0) {
                $this->info("Record with ID $id was checked and modified successfully.");
            } else {
                $this->info("Record with ID $id was checked but not modified.");
            }
        } else {
            if ($modifiedCount > 0) {
                $this->info("$modifiedCount record(s) were updated successfully.");
            } else {
                $this->info('No records were updated.');
            }
        }
    }
}
