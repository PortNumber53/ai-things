import sys
from PIL import Image
import requests
import io
from dotenv import load_dotenv, find_dotenv
import os
import time
import json

# Load environment variables from .env (and optional _extra_env) file
load_dotenv(find_dotenv())
load_dotenv(find_dotenv("_extra_env"))

# Check if a filepath argument is provided
if len(sys.argv) < 3:
    print("Please provide a filepath and a prompt as arguments.")
    sys.exit(1)

# Get the filepath from command-line argument
filepath = sys.argv[1]

MODEL_ID = os.getenv("HUGGING_FACE_MODEL", "black-forest-labs/FLUX.1-dev")
# Hugging Face deprecated `api-inference.huggingface.co` (HTTP 410).
# Use the Router endpoint instead.
HF_BASE_URL = os.getenv("HUGGING_FACE_BASE_URL", "https://router.huggingface.co")
API_URL = f"{HF_BASE_URL}/models/{MODEL_ID}"
token = os.getenv("HUGGING_FACE_TOKEN")
if not token:
    print("Missing HUGGING_FACE_TOKEN (set it in .env or _extra_env).")
    sys.exit(1)

headers = {
    "Authorization": f"Bearer {token}",
    # Nudge the API to return an image payload (when successful).
    "Accept": "image/*",
}

def _decode_json_or_text(content: bytes, content_type: str) -> str:
    # Best-effort decode to provide useful error output.
    try:
        if "application/json" in (content_type or "").lower():
            return json.dumps(json.loads(content.decode("utf-8", errors="replace")), indent=2)
    except Exception:
        pass
    return content.decode("utf-8", errors="replace")


def query_image_bytes(prompt: str, max_attempts: int = 6) -> bytes:
    """
    Call the Hugging Face inference API and return image bytes.
    Handles common non-image responses (JSON errors, model loading).
    """
    payload = {
        "inputs": prompt,
        # Ask HF to wait for model spin-up rather than returning a "loading" JSON error.
        "options": {"wait_for_model": True},
    }
    backoff = 1.0
    last_error = None
    for attempt in range(1, max_attempts + 1):
        try:
            response = requests.post(API_URL, headers=headers, json=payload, timeout=120)
        except Exception as e:
            last_error = f"request failed: {e}"
            time.sleep(backoff)
            backoff = min(backoff * 2, 10)
            continue

        content_type = response.headers.get("content-type", "")
        if response.status_code == 200 and content_type.lower().startswith("image/"):
            return response.content

        # Non-image response; capture body for debugging.
        body = _decode_json_or_text(response.content, content_type).strip()

        # If HF returns a "model is loading" response, retry with a short backoff.
        lowered = body.lower()
        if response.status_code in (503, 429) and (
            "loading" in lowered or "estimated_time" in lowered or "currently loading" in lowered
        ):
            last_error = f"model not ready (attempt {attempt}/{max_attempts}): {body[:1000]}"
            time.sleep(backoff)
            backoff = min(backoff * 2, 10)
            continue

        raise RuntimeError(
            "Hugging Face inference API did not return an image.\n"
            f"status={response.status_code} content-type={content_type or 'unknown'}\n"
            f"body:\n{body[:4000]}"
        )

    raise RuntimeError(last_error or "Hugging Face inference API failed after retries")

image_bytes = query_image_bytes(sys.argv[2])

# Open the image using PIL
image = Image.open(io.BytesIO(image_bytes))

# Save the image to the specified filepath
image.save(filepath)

print(f"Image saved successfully to: {filepath}")
