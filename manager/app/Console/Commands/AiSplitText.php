<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use App\Jobs\AiSplitSentences;

class AiSplitText extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'Ai:SplitText';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Splits a text into individual sentenses and adds TTS jobs';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $fullFilePath = '';
        AiSplitSentences::dispatch($fullFilePath)->onQueue('tts_split_text')->onConnection('rabbitmq');
    }
}
