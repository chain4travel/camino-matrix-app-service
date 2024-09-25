#!/bin/bash



if ! go build -o build/camino-matrix-app-service main.go
then
    echo "Build failed."
    exit 1
else
    echo "Build successful!"
fi
