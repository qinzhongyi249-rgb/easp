#!/bin/bash
APP_NAME="app.main"
LOG_FILE="server.log"

# Set port from argument or default to 8000
PORT=${1:-9000}
export PORT

# Find PID of the process
PID=$(ps -ef | grep "python3 -m $APP_NAME" | grep -v grep | awk '{print $2}')

if [ -n "$PID" ]; then
    echo "Stopping existing $APP_NAME (PID: $PID)..."
    kill -9 $PID
    # Wait for a moment to ensure it's gone
    sleep 2
    echo "Stopped."
else
    echo "$APP_NAME is not currently running."
fi

echo "Starting $APP_NAME on port $PORT..."
nohup python3 -m $APP_NAME > $LOG_FILE 2>&1 &
echo "$APP_NAME started on port $PORT. Check $LOG_FILE for details."
