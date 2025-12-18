import argparse
from Tiktok_uploader import uploadVideo
import os
from dotenv import load_dotenv, find_dotenv

# Load environment variables from .env (and optional _extra_env) file
load_dotenv(find_dotenv())
load_dotenv(find_dotenv("_extra_env"))

# Set up argument parser
parser = argparse.ArgumentParser(description="Upload a video to TikTok")
parser.add_argument("file", help="Path to the video file")
parser.add_argument("title", help="Title of the video")
parser.add_argument("--tags", nargs='+', default=[], help="Tags for the video")
parser.add_argument("--schedule_time", type=float, help="Schedule time for the video (optional)")

# Parse arguments
args = parser.parse_args()

# Get session_id from .env file
session_id = os.getenv("TIKTOK_SESSION_ID")
if not session_id:
    raise ValueError("TIKTOK_SESSION_ID not found in environment (.env or _extra_env)")

# Upload the video
if args.schedule_time:
    uploadVideo(session_id, args.file, args.title, args.tags, args.schedule_time, verbose=True)
else:
    uploadVideo(session_id, args.file, args.title, args.tags, verbose=True)
