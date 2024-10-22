import torch
from transformers import AutoModelForSpeechSeq2Seq, AutoProcessor, pipeline, WhisperTimeStampLogitsProcessor
from datasets import load_dataset
import json
import os
import sys
import logging

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Check if the correct number of command-line arguments are provided
if len(sys.argv) < 3:
    print("Usage: python script.py <sample_filepath> <content_id>")
    sys.exit(1)

# Extract sample filepath and content_id from command-line arguments
sample_filepath = sys.argv[1]
content_id = int(sys.argv[2])

device = "cpu"
torch_dtype = torch.float16 if torch.cuda.is_available() else torch.float32

model_id = "openai/whisper-small"

model = AutoModelForSpeechSeq2Seq.from_pretrained(
    model_id,
    torch_dtype=torch_dtype,
    low_cpu_mem_usage=True,
    use_safetensors=True,
)
model.to(device)

processor = AutoProcessor.from_pretrained(model_id)

# Add the WhisperTimeStampLogitsProcessor
# timestamp_processor = WhisperTimeStampLogitsProcessor(language="en")

pipe = pipeline(
    "automatic-speech-recognition",
    model=model,
    tokenizer=processor.tokenizer,
    feature_extractor=processor.feature_extractor,
    max_new_tokens=128,
    chunk_length_s=60,
    batch_size=8,
    return_timestamps=True,
    torch_dtype=torch_dtype,
    device=device,
    # logits_processor=[timestamp_processor],  # Add this line
)

dataset = load_dataset("distil-whisper/librispeech_long", "clean", split="validation")
sample = sample_filepath  # Use the provided sample filepath
logger.info(f'Sample filepath: {sample}')

result = pipe(sample, return_timestamps="word")
logger.info(result["text"])
logger.info(result["chunks"])

# Define the file paths to save the results
save_dir = "/output/subtitles"
os.makedirs(save_dir, exist_ok=True)
text_output_file_path = os.path.join(save_dir, "result.txt")
chunks_output_file_path = os.path.join(save_dir, "chunks.json")

# Write the result text to the file
with open(text_output_file_path, "w") as f:
    f.write(result["text"])

# Write the result chunks to a JSON file
with open(chunks_output_file_path, "w") as f:
    json.dump(result["chunks"], f)

logger.info(f"Result text saved to: {text_output_file_path}")
logger.info(f"Result chunks saved to: {chunks_output_file_path}")

def chunks_to_srt(chunks, content_id, output_file_path):
    with open(output_file_path, "w") as f:
        count = 1
        for i, chunk in enumerate(chunks):
            start_time = chunk['timestamp'][0]
            end_time = chunk['timestamp'][1]
            
            if start_time is None or end_time is None:
                logger.warning(f"Chunk {i} has None timestamp: {chunk}")
                continue  # Skip this chunk if timestamp is None

            start_time = int(start_time * 1000)  # Convert to milliseconds
            end_time = int(end_time * 1000)  # Convert to milliseconds

            text = chunk['text']

            # Format the time in HH:MM:SS,mmm
            start_time_formatted = "{:02d}:{:02d}:{:02d},{:03d}".format(
                start_time // 3600000, (start_time // 60000) % 60, (start_time // 1000) % 60, start_time % 1000
            )
            end_time_formatted = "{:02d}:{:02d}:{:02d},{:03d}".format(
                end_time // 3600000, (end_time // 60000) % 60, (end_time // 1000) % 60, end_time % 1000
            )

            f.write(str(count) + '\n')
            f.write(start_time_formatted + ' --> ' + end_time_formatted + '\n')
            f.write(text.strip() + '\n\n')
            count += 1

    logger.info(f"SRT file saved to: {output_file_path}")

# Usage:
srt_output_file_path = os.path.join(save_dir, f"transcription_{content_id}.srt")
logger.info(f"Saving SRT to {srt_output_file_path}")
chunks_to_srt(result["chunks"], content_id, srt_output_file_path)