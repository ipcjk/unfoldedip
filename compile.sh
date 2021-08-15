#! /bin/bash -xv

# Create directory
mkdir -p releases

# Compile Linux Server
echo "Compile Linux version"
GOOS=linux go build -o releases/unfolded.linux

# Compile Linux Raspberry
echo "Compile Raspberry Pi version"
GOOS=linux GOARCH=arm GOARM=5 go build -o releases/unfolded.pi

# Compile MacOS
echo "Compile MacOS version"
GOOS=darwin  go build -o releases/unfolded.darwin

# Compile MacOS
echo "Compile Windows version"
GOOS=windows go build -o releases/unfolded.exe

