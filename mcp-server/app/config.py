import os
from dotenv import load_dotenv

# Load environment variables from .env file
load_dotenv()

# OA Configuration
OA_LOGIN_ID = os.getenv("OA_LOGIN_ID")
OA_PASSWORD = os.getenv("OA_PASSWORD")

# WeCom Configuration
WECOM_CORPID = os.getenv("WECOM_CORPID")
WECOM_CORPSECRET = os.getenv("WECOM_CORPSECRET")
WECOM_AGENTID = int(os.getenv("WECOM_AGENTID", "0"))

# Feature Flags
# Define which features are enabled. Options: "oa", "wecom", "travel"
# Example: ENABLED_FEATURES="oa,wecom,travel"
ENABLED_FEATURES = [
    feature.strip()
    for feature in os.getenv("ENABLED_FEATURES", "oa,wecom,travel").split(",")
    if feature.strip()
]

# Travel / Map Configuration
TENCENT_MAP_KEY = os.getenv("TENCENT_MAP_KEY")
TENCENT_MAP_BASE_URL = os.getenv("TENCENT_MAP_BASE_URL", "https://apis.map.qq.com/ws")
TENCENT_MAP_TIMEOUT = float(os.getenv("TENCENT_MAP_TIMEOUT", "20"))

BOCHA_API_KEY = os.getenv("BOCHA_API_KEY")
BOCHA_API_URL = os.getenv("BOCHA_API_URL", "https://api.bochaai.com/v1/ai-search")
BOCHA_TIMEOUT = float(os.getenv("BOCHA_TIMEOUT", "20"))

VECTOR_DB_URL = os.getenv("VECTOR_DB_URL")
VECTOR_DB_USER = os.getenv("VECTOR_DB_USER", "root")
VECTOR_DB_PASSWORD = os.getenv("VECTOR_DB_PASSWORD")
VECTOR_DB_DATABASE = os.getenv("VECTOR_DB_DATABASE", "vector_map_cp_info_db")
VECTOR_DB_COLLECTION = os.getenv("VECTOR_DB_COLLECTION", "travel_place_knowledge")
VECTOR_DB_TIMEOUT = float(os.getenv("VECTOR_DB_TIMEOUT", "20"))
VECTOR_DB_VECTOR_DIMENSION = int(os.getenv("VECTOR_DB_VECTOR_DIMENSION", "768"))
VECTOR_DB_EMBED_MODEL = os.getenv("VECTOR_DB_EMBED_MODEL", "bge-base-zh")

# API Security
# Token required for all REST API calls. 
# Clients must send this token in the 'X-API-Token' header.
API_TOKEN = os.getenv("API_TOKEN")

# Server Configuration
PORT = int(os.getenv("PORT", "9000"))
