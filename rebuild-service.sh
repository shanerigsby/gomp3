#!/bin/bash

go build -o gomp3

if [ $? -eq 0 ]; then
    sudo systemctl restart gomp3.service
    echo "Build completed without errors. Command issued to restart gomp3.service"
else
    echo "Build failed. Service not restarted."
fi
