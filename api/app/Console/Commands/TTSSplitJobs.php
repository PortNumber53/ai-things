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
        $content = Content::where('status', 'new')->first();


        dump($content);
    }
}
