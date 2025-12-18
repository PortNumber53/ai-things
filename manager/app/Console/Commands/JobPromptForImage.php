<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use Illuminate\Support\Facades\Http;
use App\Support\ExtraEnv;

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
        {--regenerate : Regenerate the image}
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

    protected $waiting_processing_flags = [
        true => [
            'funfact_created',
        ],
        false => [
            'thumbnail_generated',
        ],
    ];

    protected $finished_processing_flags = [
        true => [
            'funfact_created',
            'wav_generated',
            'mp3_generated',
            'srt_generated',
            'thumbnail_generated',
        ],
        false => [
            'podcast_ready',
        ],
    ];

    protected $MAX_IMG_WAITING = 100;

    protected function processContent($content_id)
    {
        $current_host = config('app.hostname');

        $base_query = Content::where('type', 'gemini.payload');
        // Count how many rows are processed but waiting for upload.
        $count_query = clone $base_query;
        foreach ($this->finished_processing_flags[true] as $flag_true) {
            $count_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->finished_processing_flags[false] as $flag_false) {
            $count_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }
        $count = $count_query
            ->count();
        $this->line("NEW Count query");
        $this->dq($count_query);

        // Get the rows to be processed using $waiting_processing_flags
        $work_query = clone $base_query;
        foreach ($this->waiting_processing_flags[true] as $flag_true) {
            $work_query->whereJsonContains('meta->status->' . $flag_true, true);
        }
        foreach ($this->waiting_processing_flags[false] as $flag_false) {
            $work_query->where(function ($query) use ($flag_false) {
                $query->where('meta->status->' . $flag_false, '!=', true)
                    ->orWhereNull('meta->status->' . $flag_false);
            });
        }

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

        $regenerate = $this->option('regenerate');
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
        - Do not include any preamble, or comments, or introduction, or explanation, or commentary, or any other additional text.
        - Only output the prompt, nothing else.
        - Make sure to include the name of the place, or subject to help the AI generate an accurate image.
        """
        USER """
        $text
        """
        PROMPT;
        $this->line($prompt);

        ExtraEnv::load();
        $apiKey = env('PORTNUMBER53_API_KEY');
        if (empty($apiKey)) {
            $this->error('Missing PORTNUMBER53_API_KEY (set it in .env or _extra_env).');
            return 1;
        }

        $response = Http::timeout(300)->withHeaders([
            'X-API-key' => $apiKey,
        ])->post('https://ollama.portnumber53.com/api/generate', [
            'model' => 'llama3.3',
            'stream' => false,
            'prompt' => $prompt,
        ]);

        $response_json = $response->json();
        $body_response = trim($response_json['response'], '"');

        // Remove surrounding quotes and escape special characters for shell safety
        $sanitized_text = escapeshellarg(trim($body_response, '"\''));

        $filename = sprintf("%010d.jpg", $this->content->id);
        $full_path = config('app.output_folder') . "/images/{$filename}";

        if ($regenerate) {
            // delete file
            if (file_exists($full_path)) {
                $this->line("Deleting file: " . $full_path);
                unlink($full_path);
            }
        }
        // Loop until the file exists
        $counter = 0;
        while (!file_exists($full_path)) {

            // Execute the ./imagegeneration/image-flux.py with $full_path and $sanitized_text as the arguments
            $command = sprintf(
                'python ../imagegeneration/image-flux.py %s %s',
                escapeshellarg($full_path),
                $sanitized_text
            );
            $this->line($command);
            exec($command);

            $this->line($full_path);
            print_r($body_response);
            $this->line("");
            $this->line("Waiting for image to be generated {$counter} ...");
            sleep(2);
            $counter++;
        }
        $this->line("File exists: " . file_exists($full_path));
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
