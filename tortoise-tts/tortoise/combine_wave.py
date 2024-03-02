import psycopg2
import json
import os
import numpy as np
import soundfile as sf
from dotenv import load_dotenv
import argparse

# Load environment variables from .env file
load_dotenv()

# Get the BASE_OUTPUT_FOLDER from the environment variables
BASE_OUTPUT_FOLDER = os.getenv('BASE_OUTPUT_FOLDER', '/output/')

def get_wav_paths_from_database(content_id):
    try:
        connection = psycopg2.connect(
            host=os.getenv('DB_HOST'),
            port=os.getenv('DB_PORT'),
            database=os.getenv('DB_DATABASE'),
            user=os.getenv('DB_USERNAME'),
            password=os.getenv('DB_PASSWORD')
        )
        cursor = connection.cursor()

        cursor.execute("SELECT meta FROM contents WHERE id=%s", (content_id,))
        result = cursor.fetchone()

        cursor.close()
        connection.close()

        if result:
            meta = result[0]
            if 'filenames' in meta:
                filenames_json = meta['filenames']
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
                    return filenames
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

    for wav_path in wav_paths:
        if wav_path == '<spacer>':
            silence_samples = int(silence_duration * sample_rate)
            combined_frames.append(np.zeros((silence_samples, 1), dtype=np.float32))
            continue

        frames, sr = sf.read(wav_path, dtype='float32')

        if sample_rate is None:
            sample_rate = sr
        elif sample_rate != sr:
            raise ValueError("All WAV files must have the same sample rate.")

        num_channels = 1 if len(frames.shape) == 1 else frames.shape[1]

        if len(frames.shape) == 1:
            frames = frames.reshape(-1, 1)

        combined_frames.append(frames)

    combined_frames = np.concatenate(combined_frames, axis=0)

    sf.write(output_path, combined_frames, sample_rate)

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Combine WAV files with silence.')
    parser.add_argument('content_id', type=int, help='ID of the content to process')
    args = parser.parse_args()

    wav_paths = get_wav_paths_from_database(args.content_id)

    if not wav_paths:
        print("No filenames retrieved for the specified content ID.")
    else:
        # Construct the output path using BASE_OUTPUT_FOLDER and content_id
        output_path = os.path.join(BASE_OUTPUT_FOLDER, 'waves', f'output_combined_with_silence_{args.content_id}.wav')
        try:
            combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1)
            print(f"Combined WAV files saved to: {output_path}")
        except ValueError as e:
            print("Error:", e)
