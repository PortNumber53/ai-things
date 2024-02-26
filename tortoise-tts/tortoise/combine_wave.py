import soundfile as sf
import numpy as np

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

# Example usage:
wav_paths = [
    '/output/waves/0000000005-000-tom-e55f22a980f86367122f8541b3d414bd.wav',
    '<spacer>',
    '/output/waves/0000000005-001-tom-bc04a8a4bfd4e4f8b15a7be0b8ba8335.wav',
    '/output/waves/0000000005-002-tom-22cd10ec898c4fb51175ad477e51a4f4.wav',
    '/output/waves/0000000005-003-tom-1c13a6a50625874971e127b8b568b065.wav',
    '/output/waves/0000000005-004-tom-a19c6f414ae704a10ec3dced68522221.wav',
    '<spacer>',
    '/output/waves/0000000005-006-tom-6b946c0f7d98f4df1bbd91161156c7e0.wav',
    '/output/waves/0000000005-007-tom-f2771a39e208b74a8063e5859e47af99.wav',
    '/output/waves/0000000005-008-tom-a4e034102748fd1144d9027b99bbd5e2.wav',
    '<spacer>',
    '/output/waves/0000000005-010-tom-3436b4ef448c9c21598750bb928f4423.wav',
    '/output/waves/0000000005-011-tom-9f2756a39cf8707fdf1fce570990dd88.wav',
    '/output/waves/0000000005-012-tom-52b48794803be57183fb5c46f40fbfb1.wav',
    '<spacer>',
    '/output/waves/0000000005-014-tom-33d67fa686790b542333e4e1d3fbc772.wav',
    '/output/waves/0000000005-015-tom-51346c9c09d528a481fefcb7a3a3aefe.wav',
    '/output/waves/0000000005-016-tom-9a59c980e7e705ed8c28d8ad3b2254f3.wav',
    '<spacer>',
    '/output/waves/0000000005-018-tom-d93715ef8bf21794d7cd3fad2cdec602.wav',
    '/output/waves/0000000005-019-tom-f79074e3bd6a668158406695b288607a.wav',
    '/output/waves/0000000005-020-tom-217e0c51372b4a6bdfd3eda113f68426.wav',
    ]
output_path = '/output/waves/output_combined_with_silence.wav'
combine_wav_files_with_silence(wav_paths, output_path, silence_duration=1)  # Add 2 seconds of silence between files
