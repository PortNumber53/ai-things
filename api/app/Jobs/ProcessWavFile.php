<?php

namespace App\Jobs;

use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;

class ProcessWavFile implements ShouldQueue
{
    use Dispatchable;
    use InteractsWithQueue;
    use Queueable;
    use SerializesModels;

    protected $contentId;
    protected $hostname;

    public function __construct($contentId, $hostname)
    {
        $this->contentId = $contentId;
        $this->hostname = $hostname;
    }

    public function handle()
    {
        // Your processing logic here
    }
}
