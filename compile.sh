#! /bin/bash -xv

# Compile Linux Server
echo "Compile Linux version"
GOOS=linux go build -o unfolded.linux

# Compile Linux Raspberry
echo "Compile Raspberry Pi version"
GOOS=linux GOARCH=arm GOARM=5 go build -o unfolded.pi

# Compile MacOS
echo "Compile MacOS version"
GOOS=darwin  go build -o unfolded.darwin

# Compile MacOS
echo "Compile Windows version"
GOOS=windows go build -o unfolded.exe

