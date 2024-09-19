#!/bin/bash

go build -o build/camino-matrix-app-service main.go

if [ $? -eq 0 ]; then
    echo "Build successful!"
else
    echo "Build failed."
    exit 1
fi
