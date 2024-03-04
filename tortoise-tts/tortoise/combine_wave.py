import psycopg2
import json
import os
import numpy as np
import soundfile as sf
from dotenv import load_dotenv
import argparse
import wave
import re

# Load environment variables from .env file
load_dotenv()

# Get the BASE_OUTPUT_FOLDER from the environment variables
BASE_OUTPUT_FOLDER = os.getenv('BASE_OUTPUT_FOLDER', '/output/')

def get_wav_and_spacer_paths_from_database(content_id):
    try:
        connection = psycopg2.connect(
            host=os.getenv('DB_HOST'),
            port=os.getenv('DB_PORT'),
            database=os.getenv('DB_DATABASE'),
            user=os.getenv('DB_USERNAME'),
            password=os.getenv('DB_PASSWORD')
        )
        cursor = connection.cursor()

        # Fetch filenames from meta field
        cursor.execute("SELECT meta FROM contents WHERE id=%s", (content_id,))
        meta_result = cursor.fetchone()

        # Fetch sentences from sentences column
        cursor.execute("SELECT sentences FROM contents WHERE id=%s", (content_id,))
        sentences_result = cursor.fetchone()

        cursor.close()
        connection.close()

        if meta_result and sentences_result:
            meta = meta_result[0]
            sentences = sentences_result[0]

            if 'filenames' in meta:
                filenames_json = meta['filenames']
                sentences_json = json.loads(sentences)

                # Process spacer contents
                spacers = []
                for entry in sentences_json:
                    if entry['content'].startswith("<spacer"):
                        match = re.search(r"<spacer\s*(\d*)>", entry['content'])
                        if match:
                            spacer_length = int(match.group(1))
                        else:
                            spacer_length = 1
                        spacers.append({"content": entry['content'], "count": entry['count'], "length": spacer_length})

                # Process WAV filenames
                if isinstance(filenames_json, list):
                    filenames = []
                    for entry in sorted(filenames_json, key=lambda x: x.get('sentence_id', 0)):
                        if isinstance(entry, dict):
                            filename = entry.get('filename')
                            filename = os.path.join(BASE_OUTPUT_FOLDER, 'waves', filename)  # Construct filename with BASE_OUTPUT_FOLDER
                            if filename:
                                filenames.append(filename)
                        else:
                            print("Invalid entry in filenames_json:", entry)
                    # Merge spacer contents and filenames
                    combined_list = spacers + [{"content": filename, "count": count+len(spacers), "length": 0} for count, filename in enumerate(filenames, start=1)]

                    # Sort combined list based on count
                    combined_list.sort(key=lambda x: x['count'])

                    return combined_list
                else:
                    print("filenames_json is not a list:", filenames_json)
                    return []
            else:
                print("No filenames found in the meta.")
                return []
        else:
            print(f"No content found with ID {content_id}.")
            return []
    except psycopg2.Error as e:
        print("Error connecting to the database:", e)
        return []
    except Exception as e:
        import traceback
        traceback.print_exc()
        print("An unexpected error occurred:", e)
        return []


def combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1):
    combined_frames = []
    sample_rate = None
    total_samples = 0  # Variable to store the total number of samples

    for wav_path in wav_paths:
        frames, sr = sf.read(wav_path, dtype='float32')

        if sample_rate is None:
            sample_rate = sr
            silence_samples = int(silence_duration * sample_rate)
            combined_frames.append(np.zeros((int(silence_samples / 2), 1), dtype=np.float32))
        elif sample_rate != sr:
            raise ValueError("All WAV files must have the same sample rate.")

        num_channels = 1 if len(frames.shape) == 1 else frames.shape[1]

        if len(frames.shape) == 1:
            frames = frames.reshape(-1, 1)

        combined_frames.append(frames)

        # Accumulate the number of samples
        total_samples += frames.shape[0]

    combined_frames = np.concatenate(combined_frames, axis=0)

    sf.write(output_path, combined_frames, sample_rate)

def get_wav_duration(file_path):
    with wave.open(file_path, 'rb') as wav_file:
        # Get the number of frames and the frame rate
        num_frames = wav_file.getnframes()
        frame_rate = wav_file.getframerate()

        # Calculate the duration in seconds
        duration = num_frames / float(frame_rate)

        return duration

    # Calculate the total duration in seconds
    total_duration = total_samples / sample_rate
    print("Total Duration (seconds):", total_duration)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Combine WAV files with silence.')
    parser.add_argument('content_id', type=int, help='ID of the content to process')
    args = parser.parse_args()

    wav_paths = get_wav_and_spacer_paths_from_database(args.content_id)
    print(wav_paths)

    if not wav_paths:
        print("No filenames retrieved for the specified content ID.")
    else:
        # Construct the output path using BASE_OUTPUT_FOLDER and content_id
        output_path = os.path.join(BASE_OUTPUT_FOLDER, 'waves', f'output_combined_with_silence_{args.content_id}.wav')
        try:
            combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1)

            # Get duration of the combined WAV file
            duration = get_wav_duration(output_path)

            print(f"Combined WAV files saved to: {output_path}")
            print(f"Duration of the combined WAV file is: {duration:.2f} seconds")
        except ValueError as e:
            print("Error:", e)
