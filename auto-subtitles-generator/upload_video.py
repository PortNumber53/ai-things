#!/usr/bin/env python3

import argparse  # Importing argparse module for parsing command-line arguments
import httplib2
import os
import random
import time
import pickle

from googleapiclient.discovery import build
from googleapiclient.errors import HttpError
from googleapiclient.http import MediaFileUpload

from google_auth_oauthlib.flow import InstalledAppFlow
from google.auth.transport.requests import Request

# Explicitly tell the underlying HTTP transport library not to retry, since
# we are handling retry logic ourselves.
httplib2.RETRIES = 1

# Maximum number of times to retry before giving up.
MAX_RETRIES = 10

# Always retry when these exceptions are raised.
RETRIABLE_EXCEPTIONS = (
    httplib2.HttpLib2Error, IOError)

# Always retry when an apiclient.errors.HttpError with one of these status
# codes is raised.
RETRIABLE_STATUS_CODES = [500, 502, 503, 504]

# The CLIENT_SECRETS_FILE variable specifies the name of a file that contains
# the OAuth 2.0 information for this application, including its client_id and
# client_secret. You can acquire an OAuth 2.0 client ID and client secret from
# the Google API Console at
# https://console.cloud.google.com/.
# Please ensure that you have enabled the YouTube Data API for your project.
# For more information about using OAuth2 to access the YouTube Data API, see:
#   https://developers.google.com/youtube/v3/guides/authentication
# For more information about the client_secrets.json file format, see:
#   https://developers.google.com/api-client-library/python/guide/aaa_client_secrets
SCRIPT_DIR = os.path.abspath(os.path.dirname(__file__))

# Allow storing OAuth secrets outside the repo/deploy dir (recommended).
CLIENT_SECRETS_FILE = (
    os.environ.get("AI_THINGS_YOUTUBE_CLIENT_SECRETS_FILE")
    or os.environ.get("YOUTUBE_CLIENT_SECRETS_FILE")
    or os.path.join(SCRIPT_DIR, "client_secrets.json")
)

# Allow storing the OAuth token outside the repo/deploy dir (recommended).
TOKEN_PICKLE_FILE = (
    os.environ.get("AI_THINGS_YOUTUBE_TOKEN_FILE")
    or os.environ.get("YOUTUBE_TOKEN_FILE")
    or os.path.join(SCRIPT_DIR, "token.pickle")
)

# This OAuth 2.0 access scope allows an application to upload files to the
# authenticated user's YouTube channel, but doesn't allow other types of access.
SCOPES = ['https://www.googleapis.com/auth/youtube.upload']
YOUTUBE_API_SERVICE_NAME = "youtube"
YOUTUBE_API_VERSION = "v3"

# This variable defines a message to display if the CLIENT_SECRETS_FILE is
# missing.
MISSING_CLIENT_SECRETS_MESSAGE = """
WARNING: Please configure OAuth 2.0

To make this sample run you will need to populate the client_secrets.json file
found at:

   %s

with information from the API Console
https://console.cloud.google.com/

For more information about the client_secrets.json file format, please visit:
https://developers.google.com/api-client-library/python/guide/aaa_client_secrets
""" % os.path.abspath(os.path.join(os.path.dirname(__file__), CLIENT_SECRETS_FILE))

VALID_PRIVACY_STATUSES = ("public", "private", "unlisted")


def get_authenticated_service():
    creds = None
    if os.path.exists(TOKEN_PICKLE_FILE):
        with open(TOKEN_PICKLE_FILE, "rb") as token:
            creds = pickle.load(token)
    
    if not creds or not creds.valid:
        if creds and creds.expired and creds.refresh_token:
            creds.refresh(Request())
        else:
            flow = InstalledAppFlow.from_client_secrets_file(CLIENT_SECRETS_FILE, SCOPES)
            # On servers, run_console() is more reliable than opening a local callback server.
            # To force local_server mode, set AI_THINGS_YOUTUBE_AUTH_MODE=local_server.
            auth_mode = os.environ.get("AI_THINGS_YOUTUBE_AUTH_MODE", "").strip().lower()
            if auth_mode == "local_server":
                creds = flow.run_local_server(port=0)
            else:
                creds = flow.run_console()
        
        with open(TOKEN_PICKLE_FILE, "wb") as token:
            pickle.dump(creds, token)
    
    return build(YOUTUBE_API_SERVICE_NAME, YOUTUBE_API_VERSION, credentials=creds)


def initialize_upload(youtube, options):
    tags = None
    if options.keywords:
        tags = options.keywords.split(",")

    body = dict(
        snippet=dict(
            title=options.title,
            description=options.description,
            tags=tags,
            categoryId=options.category
        ),
        status=dict(
            privacyStatus=options.privacyStatus,
            selfDeclaredMadeForKids=False
        )
    )
    print(body)

    # Call the API's videos.insert method to create and upload the video.
    insert_request = youtube.videos().insert(
        part=",".join(body.keys()),
        body=body,
        # The chunksize parameter specifies the size of each chunk of data, in
        # bytes, that will be uploaded at a time. Set a higher value for
        # reliable connections as fewer chunks lead to faster uploads. Set a lower
        # value for better recovery on less reliable connections.
        #
        # Setting "chunksize" equal to -1 in the code below means that the entire
        # file will be uploaded in a single HTTP request. (If the upload fails,
        # it will still be retried where it left off.) This is usually a best
        # practice, but if you're using Python older than 2.6 or if you're
        # running on App Engine, you should set the chunksize to something like
        # 1024 * 1024 (1 megabyte).
        media_body=MediaFileUpload(options.file, chunksize=-1, resumable=True)
    )

    resumable_upload(insert_request)


# This method implements an exponential backoff strategy to resume a
# failed upload.
def resumable_upload(insert_request):
    response = None
    error = None
    retry = 0
    while response is None:
        try:
            print("Uploading file...")
            status, response = insert_request.next_chunk()
            if response is not None:
                if 'id' in response:
                    print("Video id '%s' was successfully uploaded." % response['id'])
                else:
                    exit("The upload failed with an unexpected response: %s" % response)
        except HttpError as e:
            if e.resp.status in RETRIABLE_STATUS_CODES:
                error = "A retriable HTTP error %d occurred:\n%s" % (e.resp.status, e.content)
            else:
                raise
        except RETRIABLE_EXCEPTIONS as e:
            error = "A retriable error occurred: %s" % e

        if error is not None:
            print(error)
            retry += 1
            if retry > MAX_RETRIES:
                exit("No longer attempting to retry.")

            max_sleep = 2 ** retry
            sleep_seconds = random.random() * max_sleep
            print("Sleeping %f seconds and then retrying..." % sleep_seconds)
            time.sleep(sleep_seconds)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("--file", required=True, help="Video file to upload")
    parser.add_argument("--title", help="Video title", default="Test Title")
    parser.add_argument("--description", help="Video description",
                        default="Test Description")
    parser.add_argument("--category", default="22",
                        help="Numeric video category. " +
                             "See https://developers.google.com/youtube/v3/docs/videoCategories/list")
    parser.add_argument("--keywords", help="Video keywords, comma separated",
                        default="")
    parser.add_argument("--privacyStatus", choices=VALID_PRIVACY_STATUSES,
                        default=VALID_PRIVACY_STATUSES[0], help="Video privacy status.")
    args = parser.parse_args()

    if not os.path.exists(args.file):
        exit("Please specify a valid file using the --file= parameter.")

    youtube = get_authenticated_service()
    try:
        initialize_upload(youtube, args)
    except HttpError as e:
        print("An HTTP error %d occurred:\n%s" % (e.resp.status, e.content))
