#!/bin/bash

for I in {pinky,legion,brain}
do
rsync -ravp --progress ${I}:/output/waves/ /output/waves/
done
