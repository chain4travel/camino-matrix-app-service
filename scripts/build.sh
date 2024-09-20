#!/bin/bash



if ! go build -o build/camino-matrix-app-service main.go
then
    echo "Build failed."
else
    echo "Build successful!"
    exit 1
fi
