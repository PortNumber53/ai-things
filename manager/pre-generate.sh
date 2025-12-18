#!/bin/bash

# Execute the commands
./artisan job:GenerateWav
./artisan job:GenerateSrt
./artisan job:GenerateMp3
./artisan job:PromptForImage --regenerate
./artisan job:GeneratePodcast
#./artisan job:UploadPodcastToYoutube --info $X
