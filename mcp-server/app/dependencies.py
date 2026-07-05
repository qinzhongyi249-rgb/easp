from fastapi import HTTPException, Security, Depends
from fastapi.security.api_key import APIKeyHeader
from app.client.bocha_client import BochaClient
from app.client.oa_client import OAClient
from app.client.tencent_map_client import TencentMapClient
from app.client.vector_db_client import VectorDBClient
from app.client.wecom_client import WeComClient
from app.services.travel_service import TravelService
from app import config

# Singleton instances
_oa_client = None
_wecom_client = None
_map_client = None
_bocha_client = None
_vector_db_client = None
_travel_service = None

# API Key Security
api_key_header = APIKeyHeader(name="X-API-Token", auto_error=True)

def verify_api_key(api_key: str = Security(api_key_header)):
    if api_key != config.API_TOKEN:
        raise HTTPException(status_code=403, detail="Invalid API Token")
    return api_key

def check_feature_enabled(feature: str):
    if feature not in config.ENABLED_FEATURES:
        raise HTTPException(status_code=403, detail=f"Feature '{feature}' is disabled in configuration")

def get_oa_client() -> OAClient:
    check_feature_enabled("oa")
    global _oa_client
    if _oa_client is None:
        _oa_client = OAClient(config.OA_LOGIN_ID, config.OA_PASSWORD)
    return _oa_client

def get_wecom_client() -> WeComClient:
    check_feature_enabled("wecom")
    global _wecom_client
    if _wecom_client is None:
        _wecom_client = WeComClient(config.WECOM_CORPID, config.WECOM_CORPSECRET, config.WECOM_AGENTID)
    return _wecom_client


def get_map_client() -> TencentMapClient:
    check_feature_enabled("travel")
    global _map_client
    if _map_client is None:
        _map_client = TencentMapClient(
            api_key=config.TENCENT_MAP_KEY,
            base_url=config.TENCENT_MAP_BASE_URL,
            timeout=config.TENCENT_MAP_TIMEOUT,
        )
    return _map_client


def get_bocha_client() -> BochaClient:
    check_feature_enabled("travel")
    global _bocha_client
    if _bocha_client is None:
        _bocha_client = BochaClient(
            api_key=config.BOCHA_API_KEY,
            api_url=config.BOCHA_API_URL,
            timeout=config.BOCHA_TIMEOUT,
        )
    return _bocha_client


def get_vector_db_client() -> VectorDBClient:
    check_feature_enabled("travel")
    global _vector_db_client
    if _vector_db_client is None:
        _vector_db_client = VectorDBClient(
            url=config.VECTOR_DB_URL,
            username=config.VECTOR_DB_USER,
            key=config.VECTOR_DB_PASSWORD,
            database_name=config.VECTOR_DB_DATABASE,
            collection_name=config.VECTOR_DB_COLLECTION,
            timeout=config.VECTOR_DB_TIMEOUT,
            dimension=config.VECTOR_DB_VECTOR_DIMENSION,
            embed_model=config.VECTOR_DB_EMBED_MODEL,
        )
    return _vector_db_client


def get_travel_service(
    map_client: TencentMapClient = Depends(get_map_client),
    bocha_client: BochaClient = Depends(get_bocha_client),
    vector_db_client: VectorDBClient = Depends(get_vector_db_client),
) -> TravelService:
    check_feature_enabled("travel")
    global _travel_service
    if _travel_service is None:
        _travel_service = TravelService(
            map_client=map_client,
            bocha_client=bocha_client,
            vector_db_client=vector_db_client,
        )
    return _travel_service

def get_travel_service_sync() -> TravelService:
    check_feature_enabled("travel")
    global _travel_service
    if _travel_service is None:
        _travel_service = TravelService(
            map_client=get_map_client(),
            bocha_client=get_bocha_client(),
            vector_db_client=get_vector_db_client(),
        )
    return _travel_service
