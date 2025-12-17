<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\Http;
use App\Support\ExtraEnv;


class ChatHiennaGPT extends Command
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'chat:HiennaGPT {query}';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Provides simplistic answers to questinos';

    /**
     * Execute the console command.
     */
    public function handle()
    {
        $query = $this->argument('query');

        ExtraEnv::load();
        $apiKey = env('PORTNUMBER53_API_KEY');
        if (empty($apiKey)) {
            $this->error('Missing PORTNUMBER53_API_KEY (set it in .env or _extra_env).');
            return 1;
        }

        $response = Http::timeout(300)->withHeaders([
            'X-API-key' => $apiKey,
        ])->post('https://ollama.portnumber53.com/api/generate', [
            'model' => 'llama3.1:8b',
            'stream' => false,
            'prompt' => <<<PROMPT
SYSTEM """
You are a politician, that answers questions always trying to avoid giving a real, or even correct answer. You also add a lot of generic definitions and circular logic to your speech. Keep your answers around 100 words. Some examples:
-When people ask you about fixing the education system, you may praise the color of the buses and be excited about their color.
-When asked about how you're going to fix the economy, you bring into the conversation social rights that have nothing to do with economics.
-When asked about inflation, you will say prices have gone up, and prices being up, makes things cost more, because inflation is high.
"""
USER """
$query
"""
PROMPT,
        ]);
        
        $response_json = $response->json();
        $body_response = $response_json['response'];
 
        print_r($body_response);

        echo "\n\n";
        var_dump(base_path());
    }
}
