#!/bin/bash

APP_NAME="easp-server"
APP_DIR="/home/workCode/easp"
PID_FILE="/tmp/easp-server.pid"
LOG_DIR="${EASP_LOG_DIR:-$APP_DIR/logs}"
LOG_FILE="$LOG_DIR/easp-server.log"
ERROR_LOG_FILE="$LOG_DIR/easp-error.log"
STDOUT_LOG_FILE="$LOG_DIR/easp-stdout.log"
VECTOR_LOG_FILE="$LOG_DIR/vector-bridge.log"
VECTOR_ERROR_LOG_FILE="$LOG_DIR/vector-bridge-error.log"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

export PORT=8082
export EASP_LOG_DIR="$LOG_DIR"

ensure_log_dir() {
    mkdir -p "$LOG_DIR"
}

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
        echo -e "${GREEN}✓ $APP_NAME is running (PID: $PID) on port 8082${NC}"
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
        pkill -9 -f "$APP_NAME" 2>/dev/null
    fi
}

start() {
    stop
    ensure_log_dir

    echo -e "${GREEN}Starting $APP_NAME on port 8082...${NC}"
    cd "$APP_DIR" || exit 1

    nohup ./$APP_NAME >> "$STDOUT_LOG_FILE" 2>&1 &
    PID=$!
    echo $PID > "$PID_FILE"

    sleep 2

    if kill -0 "$PID" 2>/dev/null; then
        echo -e "${GREEN}✓ $APP_NAME started (PID: $PID)${NC}"
        echo -e "${GREEN}  API: http://localhost:8082${NC}"
        echo -e "${GREEN}  Logs: $LOG_FILE${NC}"
    else
        echo -e "${RED}✗ $APP_NAME failed to start${NC}"
        tail -40 "$STDOUT_LOG_FILE" 2>/dev/null
        tail -40 "$ERROR_LOG_FILE" 2>/dev/null
        return 1
    fi
}

restart() {
    stop
    sleep 1
    start
}

build() {
    echo -e "${GREEN}Building $APP_NAME...${NC}"
    cd "$APP_DIR" || exit 1

    if go build -o "$APP_NAME" ./cmd/server/; then
        echo -e "${GREEN}✓ Build successful${NC}"
        restart
    else
        echo -e "${RED}✗ Build failed${NC}"
        return 1
    fi
}

logs() {
    ensure_log_dir
    case "$1" in
        error|errors)
            tail -n "${2:-200}" -f "$ERROR_LOG_FILE" "$VECTOR_ERROR_LOG_FILE" 2>/dev/null
            ;;
        vector)
            tail -n "${2:-200}" -f "$VECTOR_LOG_FILE" 2>/dev/null
            ;;
        stdout)
            tail -n "${2:-200}" -f "$STDOUT_LOG_FILE" 2>/dev/null
            ;;
        tail|server|"")
            tail -n "${2:-200}" -f "$LOG_FILE" 2>/dev/null
            ;;
        all)
            tail -n "${2:-200}" -f "$LOG_FILE" "$ERROR_LOG_FILE" "$VECTOR_LOG_FILE" "$VECTOR_ERROR_LOG_FILE" "$STDOUT_LOG_FILE" 2>/dev/null
            ;;
        *)
            echo "Usage: $0 logs [server|error|vector|stdout|all] [lines]"
            return 1
            ;;
    esac
}

errors() {
    ensure_log_dir
    echo -e "${YELLOW}== Go errors ==${NC}"
    tail -n "${1:-100}" "$ERROR_LOG_FILE" 2>/dev/null || true
    echo -e "${YELLOW}== Vector bridge errors ==${NC}"
    tail -n "${1:-100}" "$VECTOR_ERROR_LOG_FILE" 2>/dev/null || true
    echo -e "${YELLOW}== stdout fallback ==${NC}"
    tail -n "${1:-50}" "$STDOUT_LOG_FILE" 2>/dev/null || true
}

case "$1" in
    start) start ;;
    stop) stop ;;
    restart) restart ;;
    status) status ;;
    build) build ;;
    logs) shift; logs "$@" ;;
    errors|error) shift; errors "$@" ;;
    tail) shift; logs server "$@" ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|build|logs|errors|tail}"
        echo "  logs [server|error|vector|stdout|all] [lines]"
        exit 1
        ;;
esac

exit $?
