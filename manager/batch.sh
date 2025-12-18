#!/bin/bash

# Check if an argument is provided
if [ $# -eq 0 ]; then
    echo "Please provide an integer parameter."
    exit 1
fi

# Check if the argument is a valid integer
if ! [[ $1 =~ ^[0-9]+$ ]]; then
    echo "Error: Please provide a valid integer parameter."
    exit 1
fi

# Store the parameter
X=$1

# Execute the commands
./artisan job:GenerateWav $X
./artisan job:GenerateSrt $X
./artisan job:GenerateMp3 $X
./artisan job:PromptForImage --regenerate $X
./artisan job:GeneratePodcast $X
./artisan Backfill:ResponseDataToSentences $X
./artisan job:UploadPodcastToYoutube --info $X
