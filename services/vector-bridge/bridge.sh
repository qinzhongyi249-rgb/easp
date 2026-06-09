#!/bin/bash
# VectorDB Bridge Service 管理脚本

APP_NAME="vector-bridge"
APP_DIR="/home/workCode/easp/services/vector-bridge"
PID_FILE="/tmp/vector-bridge.pid"
LOG_FILE="/tmp/vector-bridge.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    else
        pgrep -f "$APP_NAME" 2>/dev/null
    fi
}

status() {
    PID=$(get_pid)
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo -e "${GREEN}✓ $APP_NAME is running (PID: $PID) on port 8083${NC}"
        return 0
    else
        echo -e "${YELLOW}✗ $APP_NAME is not running${NC}"
        return 1
    fi
}

stop() {
    PID=$(get_pid)
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo -e "${YELLOW}Stopping $APP_NAME (PID: $PID)...${NC}"
        kill "$PID" 2>/dev/null
        sleep 1
        if kill -0 "$PID" 2>/dev/null; then
            kill -9 "$PID" 2>/dev/null
        fi
        rm -f "$PID_FILE"
        echo -e "${GREEN}✓ $APP_NAME stopped${NC}"
    else
        echo -e "${YELLOW}$APP_NAME is not running${NC}"
        pkill -9 -f "server.py" 2>/dev/null
    fi
}

start() {
    stop

    echo -e "${GREEN}Starting $APP_NAME on port 8083...${NC}"
    cd "$APP_DIR"

    nohup /usr/bin/python3.12 "$APP_DIR/server.py" > "$LOG_FILE" 2>&1 &
    PID=$!
    echo $PID > "$PID_FILE"

    sleep 3

    if kill -0 "$PID" 2>/dev/null; then
        echo -e "${GREEN}✓ $APP_NAME started (PID: $PID)${NC}"
        echo -e "${GREEN}  Health: http://localhost:8083/health${NC}"
    else
        echo -e "${RED}✗ $APP_NAME failed to start${NC}"
        cat "$LOG_FILE" | tail -20
        return 1
    fi
}

restart() {
    stop
    sleep 1
    start
}

logs() {
    if [ -f "$LOG_FILE" ]; then
        tail -f "$LOG_FILE"
    else
        echo -e "${YELLOW}No log file found${NC}"
    fi
}

case "$1" in
    start) start ;;
    stop) stop ;;
    restart) restart ;;
    status) status ;;
    logs) logs ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        exit 1
        ;;
esac

exit $?
