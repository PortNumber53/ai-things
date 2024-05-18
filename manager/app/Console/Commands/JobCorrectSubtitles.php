<?php

namespace App\Console\Commands;

use App\Console\Commands\Base\BaseJobCommand;
use App\Models\Content;
use App\Classes\Parser;

class JobCorrectSubtitles extends BaseJobCommand
{
    /**
     * The name and signature of the console command.
     *
     * @var string
     */
    protected $signature = 'job:CorrectSubtitles
        {content_id? : The content ID}
        {--sleep=30 : Sleep time in seconds}
        {--queue : Process queue messages}
        ';

    /**
     * The console command description.
     *
     * @var string
     */
    protected $description = 'Fix the words in the subtitle';

    protected $content;
    protected $queue;

    protected $queue_input  = 'srt_generated';
    protected $queue_output = 'srt_fixed';

    protected $MAX_FIX_SRT_WAITING = 100;

    protected function processContent($content_id)
    {
        if (empty($content_id)) {
            $count = Content::whereJsonContains("meta->status->{$this->queue_input}", true)
                ->whereJsonDoesntContain("meta->status->{$this->queue_output}", true)
                ->count();
            if ($count >= $this->MAX_FIX_SRT_WAITING) {
                $this->info("Too many fixed SRT ($count) waiting for processing, sleeping for 60");
                sleep(60);
                exit();
            } else {
                $query = Content::where('meta->status->' . $this->queue_input, true)
                    ->where(function ($query) {
                        $query->where('meta->status->' . $this->queue_output, '!=', true)
                            ->orWhereNull('meta->status->' . $this->queue_output);
                    })
                    ->orderBy('id');

                // Print the generated SQL query
                // $this->line($query->toSql());

                // Execute the query and retrieve the first result
                $firstTrueRow = $query->first();
                if (!$firstTrueRow) {
                    $this->error("No content to process, sleeping 60 sec");
                    sleep(60);
                    exit(1);
                }
                $content_id = $firstTrueRow->id;
                // Now $firstTrueRow contains the first row ready to process
            }
        }

        $current_host = config('app.hostname');

        $this->content = Content::where('id', $content_id)->first();
        if (empty($this->content)) {
            $this->error("Content not found.");
            throw new \Exception('Content not found.');
        }


        try {
            $meta = json_decode($this->content->meta, true);

            // dump($meta['subtitles']);
            $fixed_srt = $this->fixSubtitles($meta['subtitles']);

            // dump($fixed_srt);
            die("\n\n");

            $subtitles = $meta['subtitles'];
            $srt_contents = $subtitles['srt'];
            // $vtt_contents = $subtitles['vtt'];

            $meta['subtitles']['srt'] = $this->fixSrtSubtitle($srt_contents);
            // $meta['subtitles']['vtt'] = $this->fixVttSubtitle($vtt_contents);

            $this->content->status = $this->queue_output;
            $meta["status"][$this->queue_output] = true;

            $this->content->meta = json_encode($meta);
            $this->content->save();
        } catch (\Exception $e) {
            $this->error($e->getLine());
            $this->error($e->getMessage());
            return 1;
        } finally {
            $job_payload = json_encode([
                'content_id' => $this->content->id,
                'hostname' => config('app.hostname'),
            ]);
            $this->queue->pushRaw($job_payload, $this->queue_output);

            $this->info("Job dispatched to generate the image file.");
        }
    }



    protected function fixSubtitles($input_srt)
    {

        $parser = new Parser();
        // $input_srt = file_get_contents('../subtitles.srt');
        $parser->loadString(implode($input_srt));
        // $parser->loadFile('../subtitles.srt');

        $subtitle_array = [];

        $count = 0;
        $captions = $parser->parse();
        foreach ($captions as $index => $caption) {
            $caption->text = str_replace('  ', ' ', str_replace("\n", " ", $caption->text));
            $this->line("Start Time: " . $caption->startTime);
            $this->line("End Time: " . $caption->endTime);
            $this->line("Text: " . $caption->text);

            $exploded = explode(" ", $caption->text);
            foreach ($exploded as $exploded_item) {
                $subtitle_array[] = [
                    'srt_index' => $index,
                    'count' => $count++,
                    'srt' => $exploded_item,
                ];
            }
        }

        $original_text = file_get_contents('../fixed.srt');
        dump($original_text);

        $count_original = 0;
        $lines = explode('\n', $original_text);
        foreach ($lines as $line) {
            $words = explode(" ", $line);
            foreach ($words as $word) {
                $subtitle_array[$count_original++]['orig'] = $word;
            }
        }

        $subtitle_array = $this->displaySubtitles($subtitle_array);
        $max_replaces = count($subtitle_array) * 3;

        $stop = false;
        $counter = 0;
        while (!$stop && $counter < $max_replaces) {
            $this->warn($counter);
            list($fixes_done, $subtitle_array) = $this->fixShiftSubtitlesArray($subtitle_array);
            if (!$fixes_done) {
                $this->info('----------- No more fixes');
                $stop = true;
            }
            $counter++;
        }
        $this->info('----------- After fixes');
        $subtitle_array = $this->displaySubtitles($subtitle_array);

        $this->info("ORIGINAL: $count_original    SRT: $count");
        print_r($subtitle_array[27]);

        $captions = [];
        foreach ($subtitle_array as $subtitle_item) {
            $srt_index = $subtitle_item['srt_index'];
            if (!isset($captions[$srt_index])) {
                $captions[$srt_index] = '';
            }

            $word = $subtitle_item['orig'];
            $captions[$srt_index] = trim($captions[$srt_index] . " $word");
        }
        print_r($captions);

        $fixed_str_contents = '';
        $parser_captions = $parser->parse();
        foreach ($parser_captions as $index => $caption) {
            $srt_index = $index + 1;

            $fixed_str_contents .= "{$srt_index}\n";
            $fixed_str_contents .= "{$caption->startTime} --> {$caption->endTime}\n";
            $fixed_str_contents .= "{$captions[$index]}\n";
            $fixed_str_contents .= "\n";
            $caption->text = str_replace('  ', ' ', str_replace("\n", " ", $caption->text));
        }
        print_r($fixed_str_contents);

        return $fixed_str_contents;
    }

    protected function displaySubtitles($subtitle_array = [])
    {
        $loop = 0;
        $last = count($subtitle_array);
        while (($loop < $last)) {
            $line = '';

            if (!isset($subtitle_array[$loop]['orig'])) {
                $subtitle_array[$loop]['orig'] = '';
            }
            if (!isset($subtitle_array[$loop]['srt'])) {
                $subtitle_array[$loop]['srt'] = '';
            }

            $original = $subtitle_array[$loop]['orig'];
            $srt = $subtitle_array[$loop]['srt'];

            $same_or_not = ($original === $srt) ? 'good' : 'bad';
            $subtitle_array[$loop]['status'] = $same_or_not;

            $line .= str_pad($loop, 5, '.', STR_PAD_LEFT);
            $line .= ' ' . str_pad($original, 30, ' ', STR_PAD_LEFT);
            $line .= ' ' . str_pad($srt, 30, ' ', STR_PAD_LEFT);
            $line .= ' ' . str_pad($same_or_not, 6, ' ', STR_PAD_LEFT);
            $this->line($line);

            $loop++;
        }
        return $subtitle_array;
    }

    protected function fixSubtitlesArray($subtitle_array = [])
    {
        $max = count($subtitle_array);
        $changes = false;
        for ($loop = 0; $loop < $max; $loop++) {
            $line = '';
            if (($loop > 0) && ($loop < $max)) {
                if ($subtitle_array[$loop]['status'] === 'bad') {
                    if (
                        $subtitle_array[$loop - 1]['status'] === 'good'
                        && $subtitle_array[$loop + 1]['status'] === 'good'
                    ) {
                        $changes = true;
                        $line .= "fix";
                        $subtitle_array[$loop]['srt'] = $subtitle_array[$loop]['orig'];
                        $subtitle_array[$loop]['status'] = 'fixed';
                    }
                }
            }

            if (!empty($line)) {
                $line .= "$loop - ";
                $line .= $subtitle_array[$loop]['orig'] . '=' . $subtitle_array[$loop]['srt'] . ' -> ';
                $line .= $subtitle_array[$loop]['status'] . '? ';
                $this->line("$line");
            }
        }
        return [
            $changes,
            $subtitle_array,
        ];
    }

    protected function calculateSubtitleCorrectness($subtitle_array = [])
    {
        $loop = 0;
        $counter_original = 0;
        $counter_srt = 0;
        $last = count($subtitle_array);
        while (($loop < $last)) {
            $line = '';

            if (!isset($subtitle_array[$loop]['orig'])) {
                $subtitle_array[$loop]['orig'] = '';
            }
            if (!isset($subtitle_array[$loop]['srt'])) {
                $subtitle_array[$loop]['srt'] = '';
            }

            if (!empty($subtitle_array[$loop]['orig'])) {
                $counter_original++;
            }
            if (!empty($subtitle_array[$loop]['srt'])) {
                $counter_srt++;
            }

            $original = $subtitle_array[$loop]['orig'];
            $srt = $subtitle_array[$loop]['srt'];

            $same_or_not = ($original === $srt) ? 'good' : 'bad';
            $subtitle_array[$loop]['status'] = $same_or_not;

            $line .= str_pad($loop, 5, '.', STR_PAD_LEFT);
            $line .= ' ' . str_pad($original, 30, ' ', STR_PAD_LEFT);
            $line .= ' ' . str_pad($srt, 30, ' ', STR_PAD_LEFT);
            $line .= ' ' . str_pad($same_or_not, 6, ' ', STR_PAD_LEFT);

            $loop++;
        }
        return [
            $counter_original,
            $counter_srt,
            $subtitle_array,
        ];
    }

    protected function fixShiftSubtitlesArray($subtitle_array = [])
    {
        list($counter_original, $counter_srt, $subtitle_array) = $this->calculateSubtitleCorrectness($subtitle_array);

        $max = count($subtitle_array);
        if ($counter_original == $counter_srt) {
            for ($loop = 0; $loop < $max; $loop++) {
                if ($subtitle_array[$loop]['orig'] != $subtitle_array[$loop]['srt']) {
                    $subtitle_array[$loop]['orig'] = $subtitle_array[$loop]['srt'];
                    return [
                        true,
                        $subtitle_array,
                    ];
                }
            }
            return [
                false,
                $subtitle_array,
            ];
        }

        for ($loop = 0; $loop < $max; $loop++) {
            $line = '';
            if (($loop > 0) && ($loop < $max - 2)) {
                if ($subtitle_array[$loop]['status'] === 'bad') {
                    // If the next line is also bad
                    if ($subtitle_array[$loop + 1]['status'] === 'bad') {
                        // But using the current original word and the following str work is a match we need to shift the srt
                        if (
                            $subtitle_array[$loop + 1]['orig'] === $subtitle_array[$loop + 2]['srt']
                        ) {
                            // Shift the remainder of SRT words from $loop+2 to the end and exit the function
                            for ($out = $loop + 1; $out < $max - 1; $out++) {
                                $subtitle_array[$out]['srt'] = $subtitle_array[$out + 1]['srt'];
                                $subtitle_array[$out]['srt_index'] = $subtitle_array[$out + 1]['srt_index'];
                            }
                            unset($subtitle_array[$max - 1]);

                            return [
                                true,
                                $subtitle_array,
                            ];
                        }
                    }
                }
            }
            if (($loop > 0) && ($loop < $max)) {
                if ($subtitle_array[$loop]['status'] === 'bad') {
                    if (
                        $subtitle_array[$loop - 1]['status'] === 'good'
                        && $subtitle_array[$loop + 1]['status'] === 'good'
                    ) {
                        $subtitle_array[$loop]['srt'] = $subtitle_array[$loop]['orig'];
                        $subtitle_array[$loop]['status'] = 'good';
                        return [
                            true,
                            $subtitle_array,
                        ];
                    }
                }
            }
        }
        return [
            false,
            $subtitle_array,
        ];
    }
}
