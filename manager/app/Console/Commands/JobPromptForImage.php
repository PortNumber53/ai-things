<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use Illuminate\Support\Facades\Http;

class JobPromptForImage extends BaseJobCommand
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'job:PromptForImage
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        ';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Command description';

    protected $content;
    protected $queue;

    protected $queue_input  = 'generate_image';
    protected $queue_output = 'thumbnail_generated';

    protected $flags_true = [
        'funfact_created',
    ];
    protected $flags_false = [
        'thumbnail_generated',
    ];


    protected $MAX_IMG_WAITING = 100;

    protected function processContent($content_id)
    {
        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');
        foreach ($this->flags_true as $flag_true) {
            $base_query->whereJsonContains('meta->status->' . $flag_true, true);
        }

        $count_query = clone ($base_query);
        foreach ($this->flags_false as $flag_false) {
            $count_query->whereJsonContains('meta->status->' . $flag_false, true);
        }
        $this->line("Count query");
        $this->dq($count_query);


        if (empty($content_id)) {
            $count = $count_query
                ->count();
            if ($count >= $this->MAX_IMG_WAITING) {
                $this->info("Too many IMG ($count) to process, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = $work_query
                    ->orderBy('id');

                // Print the generated SQL query
                // $this->line($query->toSql());

                // Execute the query and retrieve the first result
                $firstTrueRow = $query->first();

                $this->dq($query);
                dump($firstTrueRow);
                if (!$firstTrueRow) {
                    $this->error("No content to process, sleeping 60 sec");
                    sleep(60);
                    exit(1);
                }
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }
        $this->content = Content::where('id', $content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
            throw new \Exception('Content not found.');
        }


        // Sample Ollama call
        /*
        curl https://ollama.portnumber53.com/api/generate -d '{
        "model": "llama3.2",
        "prompt": "Why is the sky blue?",
        "stream": false
        }'*/

        $meta = json_decode($this->content->meta, true);
        print_r($meta);
        $text = isset($meta['ollama_response']['response']) ? $meta['ollama_response']['response'] : null;

        if (!$text) {
            $text = isset($meta['gemini_response']['candidates'][0]['content']['parts'][0]['text']) ? $meta['gemini_response']['candidates'][0]['content']['parts'][0]['text'] : null;
        }

        if (!$text) {
            throw new \Exception('Text not found in the meta field.');
        }

        $prompt = <<<PROMPT
        SYSTEM """
        -You are an experience designer and artist.
        -You are tasked with providing a prompt that will be used to generate an image representing the content of a text.
        -The prompt should be a short sentence or two that captures the essence of the text.
        - Do not include any preamble, or comments, or introduction, or explanation, or commentary, or any other text.
        - Only output the prompt, nothing else.
        - Make sure to include the name of the place, or subject to help the AI generate an accurate image.
        """
        USER """
        $text
        """
        PROMPT;
$this->line($prompt);

        $response = Http::timeout(300)->withHeaders([
            'X-API-key' => 'IHJeZzS6BTnoSuVoG4BLmcOe26xZHqjOyMrqQO3c4FyUUlfiMIuRijEPJspOme7',
        ])->post('https://ollama.portnumber53.com/api/generate', [
            'model' => 'llama3.1:8b',
            'stream' => false,
            'prompt' => $prompt,
        ]);

        $response_json = $response->json();
        $body_response = $response_json['response'];

        $filename = sprintf("%010d.jpg", $this->content->id);
        $full_path = config('app.output_folder') . "/images/{$filename}";

        // Loop until the file exists
        $counter = 0;
        while (!file_exists($full_path)) {
            $this->line($full_path);
            print_r($body_response);
            $this->line("");
            $this->line("Waiting for image to be generated {$counter} ...");
            sleep(10);
            $counter++;
        }
        $meta['thumbnail'] = [
            'filename' => $filename,
            'hostname' => config('app.hostname'),
        ];
        $this->content->status = $this->queue_output;
        $meta["status"][$this->queue_output] = true;
        $this->content->meta = json_encode($meta);
        // dump($this->content->meta);
        $this->content->save();
    }
}
