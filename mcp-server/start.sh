#!/bin/bash
APP_NAME="app.main"
LOG_FILE="server.log"

# Set port from argument or default to 8000
PORT=${1:-9000}
export PORT

# Check if the process is already running
PID=$(ps -ef | grep "python3 -m $APP_NAME" | grep -v grep | awk '{print $2}')

if [ -n "$PID" ]; then
    echo "$APP_NAME is already running (PID: $PID)."
else
    echo "Starting $APP_NAME on port $PORT..."
    nohup python3 -m $APP_NAME > $LOG_FILE 2>&1 &
    echo "$APP_NAME started on port $PORT. Check $LOG_FILE for details."
fi
