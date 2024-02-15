<?php

namespace App\Jobs;

use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Support\Facades\Log;
// use VladimirYuldashev\LaravelQueueRabbitMQ\Queue\RabbitMqQueue;

class AiSplitSentences implements ShouldQueue
{
    use Dispatchable;
    use InteractsWithQueue;
    use Queueable;
    use SerializesModels;

    protected $payload;

    /**
     * Create a new job instance.
     */
    public function __construct($payload)
    {
        $this->payload = $payload;
    }

    /**
     * Execute the job.
     */
    public function handle(): void
    {
        // // Publish the payload to RabbitMQ
        // $exchange = 'your_exchange_name';
        // $routingKey = 'your_routing_key';

        // $rabbitMq->publish(json_encode($this->payload), $exchange, $routingKey);

        dump($this->payload);
        Log::info('Payload published to RabbitMQ: ' . json_encode($this->payload));
    }
}
