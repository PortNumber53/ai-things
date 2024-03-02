import psycopg2
import json
import os
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

def get_wav_paths_from_database():
    try:
        connection = psycopg2.connect(
            host=os.getenv('DB_HOST'),
            port=os.getenv('DB_PORT'),
            database=os.getenv('DB_DATABASE'),
            user=os.getenv('DB_USERNAME'),
            password=os.getenv('DB_PASSWORD')
        )
        cursor = connection.cursor()

        cursor.execute("SELECT meta FROM contents")
        results = cursor.fetchall()

        cursor.close()
        connection.close()

        filenames = []
        for result in results:
            meta = result[0]
            if 'filenames' in meta:
                filenames_json = meta['filenames']
                filenames.extend([entry['filename'] for entry in filenames_json])

        if filenames:
            return filenames
        else:
            print("No filenames found in the database.")
            return []
    except psycopg2.Error as e:
        print("Error connecting to the database:", e)
        return []
    except Exception as e:
        print("An unexpected error occurred:", e)
        return []



def combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1):
    combined_frames = []
    sample_rate = None

    for wav_path in wav_paths:
        if wav_path == '<spacer>':
            # Add silence frames
            silence_samples = int(silence_duration * sample_rate)
            combined_frames.append(np.zeros((silence_samples, 1), dtype=np.float32))
            continue

        frames, sr = sf.read(wav_path, dtype='float32')

        # Store sample rate
        if sample_rate is None:
            sample_rate = sr
        elif sample_rate != sr:
            raise ValueError("All WAV files must have the same sample rate.")

        # Check if the audio is mono or stereo
        num_channels = 1 if len(frames.shape) == 1 else frames.shape[1]

        # Reshape frames if necessary
        if len(frames.shape) == 1:
            frames = frames.reshape(-1, 1)

        # Append frames from the current WAV file
        combined_frames.append(frames)

    # Concatenate frames into a single array
    combined_frames = np.concatenate(combined_frames, axis=0)

    # Save the combined frames to a new WAV file
    sf.write(output_path, combined_frames, sample_rate)

if __name__ == "__main__":
    # Get wav_paths from the database
    wav_paths = get_wav_paths_from_database()

    # Check if any filenames were retrieved
    if not wav_paths:
        print("No filenames retrieved from the database.")
    else:
        # Proceed with combining WAV files
        output_path = '/output/waves/output_combined_with_silence.wav'
        combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1)
