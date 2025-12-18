import sys
from PIL import Image
import requests
import io
from dotenv import load_dotenv, find_dotenv
import os

# Load environment variables from .env (and optional _extra_env) file
load_dotenv(find_dotenv())
load_dotenv(find_dotenv("_extra_env"))

# Check if a filepath argument is provided
if len(sys.argv) < 3:
    print("Please provide a filepath and a prompt as arguments.")
    sys.exit(1)

# Get the filepath from command-line argument
filepath = sys.argv[1]

API_URL = "https://api-inference.huggingface.co/models/black-forest-labs/FLUX.1-dev"
token = os.getenv("HUGGING_FACE_TOKEN")
if not token:
    print("Missing HUGGING_FACE_TOKEN (set it in .env or _extra_env).")
    sys.exit(1)

headers = {"Authorization": f"Bearer {token}"}

def query(payload):
    response = requests.post(API_URL, headers=headers, json=payload)
    return response.content

image_bytes = query({
    "inputs": sys.argv[2],
})

# Open the image using PIL
image = Image.open(io.BytesIO(image_bytes))

# Save the image to the specified filepath
image.save(filepath)

print(f"Image saved successfully to: {filepath}")
