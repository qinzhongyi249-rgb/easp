#!/bin/bash
APP_NAME="app.main"

# Find PID of the process
PID=$(ps -ef | grep "python3 -m $APP_NAME" | grep -v grep | awk '{print $2}')

if [ -n "$PID" ]; then
    echo "Stopping $APP_NAME (PID: $PID)..."
    kill -9 $PID
    echo "$APP_NAME stopped."
else
    echo "$APP_NAME is not running."
fi
