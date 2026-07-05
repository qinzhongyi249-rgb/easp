import logging
import sys
import os
from logging.handlers import TimedRotatingFileHandler

# Ensure logs directory exists
LOG_DIR = os.path.join(os.getcwd(), "logs")
if not os.path.exists(LOG_DIR):
    os.makedirs(LOG_DIR)

LOG_FILE = os.path.join(LOG_DIR, "app.log")

_is_configured = False

def setup_logging():
    global _is_configured
    if _is_configured:
        return

    root_logger = logging.getLogger()
    root_logger.setLevel(logging.INFO)

    # Formatter
    formatter = logging.Formatter(
        '%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )

    # Remove existing handlers to avoid duplicates if re-initialized
    if root_logger.hasHandlers():
        root_logger.handlers.clear()

    # Console Handler
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setFormatter(formatter)
    root_logger.addHandler(console_handler)

    # File Handler (Daily Rotation, keep 30 days)
    file_handler = TimedRotatingFileHandler(
        LOG_FILE, when="midnight", interval=1, backupCount=30, encoding="utf-8"
    )
    file_handler.setFormatter(formatter)
    root_logger.addHandler(file_handler)

    _is_configured = True

def get_logger(name=None):
    if not _is_configured:
        setup_logging()
    return logging.getLogger(name)

# Initialize logging on import
setup_logging()

# Export a default logger for convenience
logger = get_logger("app")
