import argparse
import ffmpeg
import os
from io import BytesIO, StringIO
from utils import write_vtt, write_srt
import whisper

DEVICE = "cpu"  # Force using CPU for inference

loaded_model = whisper.load_model("small", device=DEVICE)

def inference(loaded_model, input_file, task):
    save_dir = "output"
    os.makedirs(save_dir, exist_ok=True)

    with open(f"{save_dir}/input.mp3", "wb") as f:
        f.write(input_file.read())

    audio = ffmpeg.input(f"{save_dir}/input.mp3")
    audio = ffmpeg.output(audio, f"{save_dir}/output.wav", acodec="pcm_s16le", ac=1, ar="16k")
    ffmpeg.run(audio, overwrite_output=True)

    if task == "Transcribe":
        options = dict(task="transcribe", best_of=5)
        results = loaded_model.transcribe(f"{save_dir}/output.wav", **options)
    elif task == "Translate":
        options = dict(task="translate", best_of=5)
        results = loaded_model.transcribe(f"{save_dir}/output.wav", **options)
    else:
        raise ValueError("Task not supported")

    vtt = get_subs(results["segments"], "vtt", 80, save_dir)
    srt = get_subs(results["segments"], "srt", 80, save_dir)

    save_transcription(results["text"], save_dir)  # Save transcribed text to a file in the output folder

    return results["text"], vtt, srt, results["language"]

def get_subs(segments, format, max_line_width, save_dir):
    if format == 'vtt':
        file_path = f"{save_dir}/transcription.vtt"
    elif format == 'srt':
        file_path = f"{save_dir}/transcription.srt"
    else:
        raise Exception("Unknown format " + format)

    with open(file_path, "w") as file:
        if format == 'vtt':
            write_vtt(segments, file=file, maxLineWidth=max_line_width)
        elif format == 'srt':
            write_srt(segments, file=file, maxLineWidth=max_line_width)

    # Since the function is expected to return the content, let's read it back from the file
    with open(file_path, "r") as file:
        content = file.read()

    return content

def save_transcription(transcription_text, save_dir):
    with open(f"{save_dir}/transcription.txt", "w") as file:
        file.write(transcription_text)

def main(input_file_path, task):
    with open(input_file_path, "rb") as f:
        input_file = BytesIO(f.read())

    results = inference(loaded_model, input_file, task)

    # Process the results and save transcripts

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Auto Transcriber Command Line App')
    parser.add_argument('input_file', type=str, help='Path to the input audio file')
    parser.add_argument('task', type=str, choices=['Transcribe', 'Translate'], help='Select Task: Transcribe or Translate')

    args = parser.parse_args()

    main(args.input_file, args.task)