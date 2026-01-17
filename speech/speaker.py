import torch
from TTS.api import TTS

OUTPUT_PATH = "/output/waves/"
# Get device
device = "cuda" if torch.cuda.is_available() else "cpu"

# List available 🐸TTS models
print(TTS().list_models())

# Init TTS
# tts = TTS("tts_models/multilingual/multi-dataset/xtts_v2").to(device)

# # Run TTS
# # ❗ Since this model is multi-lingual voice cloning model, we must set the target speaker_wav and language
# # Text to speech list of amplitude values as output
# wav = tts.tts(text="Hello world!", speaker_wav="my/cloning/audio.wav", language="en")
# # Text to speech to a file
# tts.tts_to_file(text="Hello world!", speaker_wav="my/cloning/audio.wav", language="en", file_path="output.wav")



# Init TTS with the target model name
tts = TTS(model_name="tts_models/en/jenny/jenny", progress_bar=True)
# Run TTS
tts.tts_to_file(text="The Purple Emperor,  A Dazzling Butterfly with a Royal Heritage.", file_path=f'{OUTPUT_PATH}/speech.wav')
