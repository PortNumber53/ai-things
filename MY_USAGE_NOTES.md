# Usage

##Combine the generate wav files

```
python combine_wave.py 182



ffmpeg -i audio.wav -acodec libmp3lame audio.mp3


```

## Generate subtitles

```
cd auto-subtitles-generator
conda activate subtitle
python subtitle.py /output/waves/output_combined_with_silence_182.wav Transcribe 182
```

Prompt to fix subtitles

```
I ran a script to automatically generate subtitles for my text, and I want you to fix eventual mistakes that happened. I will give you the original text and the SRT file. Please fix mistakes in the SRT file and present the entire file with fixes. You only need to putput the fixed SRT file, no comments about it.

this is the original text
TITLE: Spaghettification: The Bizarre Phenomenon in Black Holes\n\nCONTENT:\n\nImagine yourself standing near a massive black hole. As you approach the event horizon, an intriguing phenomenon called \"spaghettification\" takes hold. This peculiar process involv

and this is the generate SRT file we need to fix:
```

## To play mp3

```
yay -S sox
```

## PIPER

```
yay -S espeak-ng

echo "TEST" | piper --debug --model ljspeech.onnx -c ljspeech.onnx.json  --output_file /output/waves/audio-45266.wav && play /output/waves/audio-45266.wav
```



Stable Difusion
```
yay -S bc
./webui.sh --listen --api --use-cpu GFPGAN BSRGAN ESRGAN SCUNet CodeFormer --all --lowvram
```



## Queries
```
SELECT
  COUNT(*) FILTER (WHERE meta->'status'->>'thumbnail_generated' = 'true') AS thumb_generated_count,
  COUNT(*) FILTER (WHERE meta->'status'->>'mp3_generated' = 'true') AS mp3_generated_count,
  COUNT(*) FILTER (WHERE meta->'status'->>'srt_generated' = 'true') AS srt_generated_count,
  COUNT(*) FILTER (WHERE meta->'status'->>'srt_fixed' = 'true') AS srt_fixed_count,
  COUNT(*) FILTER (WHERE meta->'status'->>'wav_generated' = 'true') AS wav_generated_count,
  COUNT(*) FILTER (WHERE meta->'status'->>'funfact_created' = 'true') AS funfact_created_count
FROM
  contents;
````
