#!/bin/bash

# Define paths
SLEEPER_PATH="./sleeper" # Replace with the path to your sleeper project
RHYTHM_PATH="./circadian_rhythm"     # Replace with the path to your waker project
OUTPUT_DIR="./bin"    # Replace with the path where you want to save the built binaries

# Check if output directory exists, create if not
[ ! -d "$OUTPUT_DIR" ] && mkdir -p "$OUTPUT_DIR"

# Building sleeper for x64 architecture
echo "Building sleeper for x64 architecture..."
env GOOS=linux GOARCH=amd64 go1.21.4 build -o "$OUTPUT_DIR/sleeper" $SLEEPER_PATH

# Building waker for Raspberry Pi (ARMv8)
echo "Building circadian rhythm for Raspberry Pi..."
# For Raspberry Pi 3 (ARMv8)
env GOOS=linux GOARCH=arm64 go1.21.4 build -o "$OUTPUT_DIR/circadian_rhythm" $RHYTHM_PATH
# Uncomment the next line and comment the above line for Raspberry Pi Zero or Pi 1 (ARMv6)
# env GOOS=linux GOARCH=arm GOARM=6 go build -o "$OUTPUT_DIR/waker" $WAKER_PATH

echo "Build process complete."
