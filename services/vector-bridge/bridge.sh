#!/bin/bash
# VectorDB Bridge Service 管理脚本

APP_NAME="vector-bridge"
APP_DIR="/home/workCode/easp/services/vector-bridge"
PROJECT_DIR="/home/workCode/easp"
PID_FILE="/tmp/vector-bridge.pid"
LOG_DIR="${EASP_LOG_DIR:-$PROJECT_DIR/logs}"
LOG_FILE="$LOG_DIR/vector-bridge.log"
ERROR_LOG_FILE="$LOG_DIR/vector-bridge-error.log"
STDOUT_LOG_FILE="$LOG_DIR/vector-bridge-stdout.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

export EASP_LOG_DIR="$LOG_DIR"

ensure_log_dir() {
    mkdir -p "$LOG_DIR"
}

get_pid() {
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
            echo "$PID"
            return
        fi
    fi
    # Only match the actual Python server process, not this shell script/command line.
    pgrep -f "$APP_DIR/server.py" 2>/dev/null
}

status() {
    PID=$(get_pid)
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo -e "${GREEN}✓ $APP_NAME is running (PID: $PID) on port 8083${NC}"
        echo -e "${GREEN}  Logs: $LOG_DIR${NC}"
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
        pkill -9 -f "$APP_DIR/server.py" 2>/dev/null
    fi
}

start() {
    stop
    ensure_log_dir

    echo -e "${GREEN}Starting $APP_NAME on port 8083...${NC}"
    cd "$APP_DIR" || exit 1

    # Load local secrets/config if present (VECTORDB_KEY, VECTORDB_COLLECTION, etc.)
    if [ -f "$APP_DIR/.env" ]; then
        set -a
        source "$APP_DIR/.env"
        set +a
    fi

    nohup /usr/bin/python3.12 "$APP_DIR/server.py" >> "$STDOUT_LOG_FILE" 2>&1 &
    PID=$!
    echo $PID > "$PID_FILE"

    sleep 3

    if kill -0 "$PID" 2>/dev/null; then
        echo -e "${GREEN}✓ $APP_NAME started (PID: $PID)${NC}"
        echo -e "${GREEN}  Health: http://localhost:8083/health${NC}"
        echo -e "${GREEN}  Logs: $LOG_FILE${NC}"
    else
        echo -e "${RED}✗ $APP_NAME failed to start${NC}"
        tail -20 "$STDOUT_LOG_FILE" 2>/dev/null
        tail -20 "$ERROR_LOG_FILE" 2>/dev/null
        return 1
    fi
}

restart() {
    stop
    sleep 1
    start
}

logs() {
    ensure_log_dir
    case "$1" in
        error|errors)
            tail -n "${2:-200}" -f "$ERROR_LOG_FILE" 2>/dev/null
            ;;
        stdout)
            tail -n "${2:-200}" -f "$STDOUT_LOG_FILE" 2>/dev/null
            ;;
        all)
            tail -n "${2:-200}" -f "$LOG_FILE" "$ERROR_LOG_FILE" "$STDOUT_LOG_FILE" 2>/dev/null
            ;;
        ""|server|tail)
            tail -n "${2:-200}" -f "$LOG_FILE" 2>/dev/null
            ;;
        *)
            echo "Usage: $0 logs [server|error|stdout|all] [lines]"
            return 1
            ;;
    esac
}

case "$1" in
    start) start ;;
    stop) stop ;;
    restart) restart ;;
    status) status ;;
    logs) shift; logs "$@" ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        echo "  logs [server|error|stdout|all] [lines]"
        exit 1
        ;;
esac

exit $?
