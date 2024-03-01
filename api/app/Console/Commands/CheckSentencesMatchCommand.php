<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Models\Content;

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
            $sentences = collect(json_decode($record->sentences, true))->where('content', '!=', '<spacer>');

            $meta = optional(json_decode($record->meta));
            $filenames = collect($meta->filenames ?? []);

            if ($sentences->count() === $filenames->count()) {
                $allMatch = true;
                foreach ($sentences as $sentence) {
                    $sentenceId = $sentence['count'];
                    $match = $filenames->contains('sentence_id', $sentenceId);
                    if (!$match) {
                        $allMatch = false;
                        break;
                    }
                }

                if ($allMatch && $record->type !== 'gemini.wav_ready') {
                    $record->type = 'gemini.wav_ready';
                    $record->save();
                    $modifiedCount++;
                }
            }
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
