import whisper
import ffmpeg
from utils import write_vtt, write_srt
import requests
from typing import Iterator
from io import StringIO
import pathlib
import os
import torch
from zipfile import ZipFile
import base64
import re

DEVICE = "cuda" if torch.cuda.is_available() else "cpu"
loaded_model = whisper.load_model("small", device=DEVICE)

LOCAL_DIR = pathlib.Path(__file__).parent.absolute() / "local_audio"
LOCAL_DIR.mkdir(exist_ok=True)
save_dir = LOCAL_DIR / "output"
save_dir.mkdir(exist_ok=True)

def inferecence(loaded_model, audio_file, task):
    audio = ffmpeg.input(audio_file)
    audio = ffmpeg.output(audio, f"{save_dir}/output.wav", acodec="pcm_s16le", ac=1, ar="16k")
    ffmpeg.run(audio, overwrite_output=True)
    if task == "Transcribe":
        options = dict(task="transcribe", best_of=5)
        results = loaded_model.transcribe(f"{save_dir}/output.wav", **options)
        vtt = getSubs(results["segments"], "vtt", 80)
        srt = getSubs(results["segments"], "srt", 80)
        lang = results["language"]
        return results["text"], vtt, srt, lang
    elif task == "Translate":
        options = dict(task="translate", best_of=5)
        results = loaded_model.transcribe(f"{save_dir}/output.wav", **options)
        vtt = getSubs(results["segments"], "vtt", 80)
        srt = getSubs(results["segments"], "srt", 80)
        lang = results["language"]
        return results["text"], vtt, srt, lang
    else:
        raise ValueError("Task not supported")

def getSubs(segments: Iterator[dict], format: str, maxLineWidth: int) -> str:
    segmentStream = StringIO()

    if format == 'vtt':
        write_vtt(segments, file=segmentStream, maxLineWidth=maxLineWidth)
    elif format == 'srt':
        write_srt(segments, file=segmentStream, maxLineWidth=maxLineWidth)
    else:
        raise Exception("Unknown format " + format)

    segmentStream.seek(0)
    return segmentStream.read()

def main():
    import argparse
    parser = argparse.ArgumentParser(description="Auto Transcriber")
    parser.add_argument("audio_file", help="Path to the audio file")
    parser.add_argument("--task", choices=["Transcribe", "Translate"], default="Transcribe", help="Select Task")
    args = parser.parse_args()

    audio_file = args.audio_file
    task = args.task

    if os.path.exists(audio_file):
        results = inferecence(loaded_model, audio_file, task)
        # You can handle the results here, like saving transcripts, etc.
    else:
        print("Error: Audio file not found.")

if __name__ == "__main__":
    main()
