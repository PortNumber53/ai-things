import os
import json
import argparse
import wave
import re
import numpy as np
import soundfile as sf
import psycopg2
from my_database import get_database_connection
from dotenv import load_dotenv  # Import load_dotenv function

# Load environment variables from .env file
load_dotenv()

BASE_OUTPUT_FOLDER = os.getenv('BASE_OUTPUT_FOLDER', '/output')


def get_wav_and_spacer_paths_from_database(content_id):
    try:
        connection = get_database_connection()
        cursor = connection.cursor()

        cursor.execute("SELECT meta FROM contents WHERE id=%s", (content_id,))
        meta_result = cursor.fetchone()

        cursor.execute("SELECT sentences FROM contents WHERE id=%s", (content_id,))
        sentences_result = cursor.fetchone()

        cursor.close()
        connection.close()

        if meta_result and sentences_result:
            meta = meta_result[0]
            sentences = sentences_result[0]

            if 'filenames' in meta:
                filenames_json = meta['filenames']

                spacers = []
                for entry in sentences:
                    if entry['content'].startswith("<spacer"):
                        match = re.search(r"<spacer\s*(\d*)>", entry['content'])
                        spacer_length = int(match.group(1)) if match else 1
                        spacers.append({"content": entry['content'], "count": entry['count'], "length": spacer_length})

                if isinstance(filenames_json, list):
                    filenames = []
                    for entry in sorted(filenames_json, key=lambda x: x.get('sentence_id', 0)):
                        if isinstance(entry, dict):
                            filename = entry.get('filename')
                            filename = os.path.join(BASE_OUTPUT_FOLDER, '/waves', filename)
                            if filename:
                                filenames.append(filename)
                        else:
                            print("Invalid entry in filenames_json:", entry)
                    combined_list = spacers + [{"filename": filename, "count": count+len(spacers), "length": 0} for count, filename in enumerate(filenames, start=1)]
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


def combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1):
    combined_frames = []
    sample_rate = None
    total_samples = 0

    for wav_path in wav_paths:
        if isinstance(wav_path, dict) and 'filename' in wav_path:
            frames, sample_rate = sf.read(wav_path['filename'], dtype='float32')
            break

    if sample_rate is None:
        raise ValueError("No WAV file with a valid filename found.")

    silence_samples = int(silence_duration * sample_rate)
    combined_frames.append(np.zeros((int(silence_samples / 2), 1), dtype=np.float32))

    for wav_path in wav_paths:
        if isinstance(wav_path, dict) and 'filename' in wav_path:
            filename = wav_path['filename']
            if os.path.exists(filename):
                print(f'Adding {filename} to the output file.')
                frames, sr = sf.read(filename, dtype='float32')
                if sample_rate != sr:
                    raise ValueError("All WAV files must have the same sample rate.")
                num_channels = 1 if len(frames.shape) == 1 else frames.shape[1]
                if len(frames.shape) == 1:
                    frames = frames.reshape(-1, 1)
                combined_frames.append(frames)
                total_samples += frames.shape[0]
            else:
                print(f'File {filename} does not exist.')

    combined_frames = np.concatenate(combined_frames, axis=0)
    sf.write(output_path, combined_frames, sample_rate)
    duration = get_wav_duration(output_path)

    return output_path, duration


def get_wav_duration(file_path):
    with wave.open(file_path, 'rb') as wav_file:
        num_frames = wav_file.getnframes()
        frame_rate = wav_file.getframerate()
        duration = num_frames / float(frame_rate)
        return duration

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Combine WAV files with silence.')
    parser.add_argument('content_id', type=int, help='ID of the content to process')
    args = parser.parse_args()

    wav_paths = get_wav_and_spacer_paths_from_database(args.content_id)
    if not wav_paths:
        print("No filenames retrieved for the specified content ID.")
    else:
        output_path = os.path.join(BASE_OUTPUT_FOLDER, '/waves', f'output_combined_with_silence_{args.content_id}.wav')
        try:
            output_path, duration = combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1)
            print(f"Combined WAV files saved to: {output_path}")
            print(f"Duration of the combined WAV file is: {duration:.2f} seconds")

            try:
                connection = get_database_connection()
                cursor = connection.cursor()
                cursor.execute("UPDATE contents SET meta = jsonb_set(meta, '{combined_wav}', %s) WHERE id=%s",
                               (json.dumps({"filename": output_path, "duration": duration}), args.content_id))
                connection.commit()
                cursor.close()
                connection.close()
                print("Meta field updated successfully.")
            except psycopg2.Error as e:
                print("Error connecting to the database:", e)
            except Exception as e:
                print("An unexpected error occurred while updating the meta field:", e)
        except ValueError as e:
            print("Error:", e)
