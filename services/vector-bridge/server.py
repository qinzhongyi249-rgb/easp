#!/usr/bin/env python3
"""
EASP VectorDB Bridge Service
腾讯云向量数据库 HTTP 桥接服务 - 使用内置 Embedding 模型
Port: 8083
"""

import json
import logging
import os
import time
import traceback
from logging.handlers import RotatingFileHandler
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse

import tcvectordb
from tcvectordb.model.enum import ReadConsistency, FieldType, IndexType, MetricType
from tcvectordb.model.index import Index, VectorIndex, FilterIndex, HNSWParams
from tcvectordb.model.document import Document
from tcvectordb.rpc.client.stub import Embedding as EmbeddingConfig

# ========== 配置 ==========
VECTORDB_URL = os.getenv("VECTORDB_URL", "http://bj-vdb-c07b6009.sql.tencentcdb.com:8100")
VECTORDB_USER = os.getenv("VECTORDB_USER", "root")
VECTORDB_KEY = os.getenv("VECTORDB_KEY", "")
DEFAULT_DATABASE = os.getenv("VECTORDB_DATABASE", "easp_memory")
DEFAULT_COLLECTION = os.getenv("VECTORDB_COLLECTION", "memories")
# 使用腾讯云向量数据库内置 Embedding 模型
# 推荐模型: bge-large-zh-v1.5 (中文, 1024维, MTEB排名靠前)
# 可选: bge-base-zh-v1.5 (中文, 768维, 性能优先), BAAI/bge-m3 (多语言, 1024维, 长文本8K)
EMBEDDING_MODEL_NAME = "bge-large-zh-v1.5"
EMBEDDING_FIELD = "content"       # 原始文本字段名
EMBEDDING_VECTOR_FIELD = "vector" # 向量字段名(固定)
EMBEDDING_DIMENSION = 1024       # bge-large-zh-v1.5 维度
PORT = 8083
LOG_DIR = os.getenv("EASP_LOG_DIR", "/home/workCode/easp/logs")

SENSITIVE_KEYS = ("authorization", "access_token", "refresh_token", "api_key", "password", "secret", "credential", "cookie", "token", "key")

def _redact_value(key, value):
    if any(k in str(key).lower() for k in SENSITIVE_KEYS):
        return "[REDACTED]"
    if isinstance(value, dict):
        return {k: _redact_value(k, v) for k, v in value.items()}
    if isinstance(value, list):
        return [_redact_value(key, v) for v in value]
    return value

def setup_logging():
    os.makedirs(LOG_DIR, exist_ok=True)
    logger = logging.getLogger("vector_bridge")
    logger.setLevel(logging.INFO)
    logger.handlers.clear()
    formatter = logging.Formatter('%(message)s')
    app_handler = RotatingFileHandler(os.path.join(LOG_DIR, "vector-bridge.log"), maxBytes=50 * 1024 * 1024, backupCount=5, encoding="utf-8")
    app_handler.setFormatter(formatter)
    err_handler = RotatingFileHandler(os.path.join(LOG_DIR, "vector-bridge-error.log"), maxBytes=50 * 1024 * 1024, backupCount=5, encoding="utf-8")
    err_handler.setFormatter(formatter)
    err_handler.setLevel(logging.ERROR)
    console = logging.StreamHandler()
    console.setFormatter(formatter)
    logger.addHandler(app_handler)
    logger.addHandler(err_handler)
    logger.addHandler(console)
    return logger

logger = setup_logging()

def log_event(level, message, **fields):
    entry = {
        "time": time.strftime('%Y-%m-%dT%H:%M:%S%z'),
        "level": level,
        "module": "vector-bridge",
        "message": message,
    }
    entry.update({k: _redact_value(k, v) for k, v in fields.items()})
    line = json.dumps(entry, ensure_ascii=False, default=str)
    getattr(logger, level if level in ("info", "warning", "error") else "info")(line)

# ========== 向量数据库客户端 ==========
client = None

def get_client():
    global client
    if client is None:
        client = tcvectordb.RPCVectorDBClient(
            url=VECTORDB_URL,
            username=VECTORDB_USER,
            key=VECTORDB_KEY,
            read_consistency=ReadConsistency.EVENTUAL_CONSISTENCY,
            timeout=30,
        )
    return client

# ========== HTTP处理器 ==========
class VectorDBHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        log_event("info", "http access", client_ip=self.client_address[0], request=format % args)

    def _send_json(self, data, status=200):
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')
        self.end_headers()
        self.wfile.write(json.dumps(data, ensure_ascii=False).encode())

    def _read_body(self):
        content_length = int(self.headers.get('Content-Length', 0))
        if content_length == 0:
            return {}
        body = self.rfile.read(content_length)
        return json.loads(body)

    def _handle_options(self):
        self.send_response(200)
        self.send_header('Access-Control-Allow-Origin', '*')
        self.send_header('Access-Control-Allow-Methods', 'GET, POST, OPTIONS')
        self.send_header('Access-Control-Allow-Headers', 'Content-Type')
        self.end_headers()

    def do_GET(self):
        path = urlparse(self.path).path
        if path == '/health':
            self._send_json({
                "status": "healthy",
                "service": "vector-bridge",
                "embedding": "bge-large-zh-v1.5",
                "dimension": EMBEDDING_DIMENSION,
            })
        elif path == '/api/embedding/models':
            # 返回当前可用的 Embedding 模型信息
            self._send_json({
                "code": 0,
                "data": {
                    "current": "bge-large-zh-v1.5",
                    "dimension": EMBEDDING_DIMENSION,
                    "field": EMBEDDING_FIELD,
                    "available_models": [
                        {"name": "bge-large-zh-v1.5", "language": "中文", "dimension": 1024, "max_tokens": 512, "recommended": True},
                        {"name": "bge-base-zh-v1.5", "language": "中文", "dimension": 768, "max_tokens": 512},
                        {"name": "BAAI/bge-m3", "language": "多语言", "dimension": 1024, "max_tokens": 8192},
                    ]
                }
            })
        else:
            self._send_json({"error": "Not found"}, 404)

    def do_OPTIONS(self):
        self._handle_options()

    def do_POST(self):
        path = urlparse(self.path).path
        start = time.time()
        try:
            body = self._read_body()
            log_event("info", "request started", path=path, client_ip=self.client_address[0])
            if path == '/api/database/list':
                self._handle_list_databases(body)
            elif path == '/api/database/create':
                self._handle_create_database(body)
            elif path == '/api/collection/list':
                self._handle_list_collections(body)
            elif path == '/api/collection/create':
                self._handle_create_collection(body)
            elif path == '/api/document/insert':
                self._handle_insert(body)
            elif path == '/api/document/search':
                self._handle_search(body)
            elif path == '/api/document/delete':
                self._handle_delete(body)
            elif path == '/api/embedding':
                self._handle_embedding(body)
            else:
                self._send_json({"error": f"Unknown endpoint: {path}"}, 404)
        except Exception as e:
            log_event("error", "request failed", path=path, client_ip=self.client_address[0], duration_ms=int((time.time() - start) * 1000), error=str(e), traceback=traceback.format_exc())
            self._send_json({"error": str(e)}, 500)
        finally:
            log_event("info", "request finished", path=path, client_ip=self.client_address[0], duration_ms=int((time.time() - start) * 1000))

    # ========== Database操作 ==========
    def _handle_list_databases(self, body):
        c = get_client()
        db_list = c.list_databases()
        databases = []
        for db in db_list:
            name = db.database_name if hasattr(db, 'database_name') else str(db)
            db_type = db.db_type if hasattr(db, 'db_type') else 'unknown'
            databases.append({"name": name, "type": str(db_type)})
        self._send_json({"code": 0, "data": databases})

    def _handle_create_database(self, body):
        name = body.get('name', DEFAULT_DATABASE)
        c = get_client()
        try:
            c.create_database(name)
            self._send_json({"code": 0, "message": f"Database {name} created"})
        except Exception as e:
            if "exist" in str(e).lower():
                self._send_json({"code": 0, "message": f"Database {name} already exists"})
            else:
                raise

    # ========== Collection操作 ==========
    def _handle_list_collections(self, body):
        db_name = body.get('database', DEFAULT_DATABASE)
        c = get_client()
        db = c.database(db_name)
        coll_list = db.list_collections()
        collections = []
        for coll in coll_list:
            name = coll.collection_name if hasattr(coll, 'collection_name') else str(coll)
            collections.append({"name": name})
        self._send_json({"code": 0, "data": collections})

    def _handle_create_collection(self, body):
        db_name = body.get('database', DEFAULT_DATABASE)
        coll_name = body.get('collection', DEFAULT_COLLECTION)
        use_embedding = body.get('use_embedding', True)  # 默认启用 Embedding
        dimension = body.get('dimension', EMBEDDING_DIMENSION)

        c = get_client()
        db = c.database(db_name)
        try:
            index = Index()
            # 使用 Embedding 时，维度由模型决定，无需手动指定
            index.add(VectorIndex(
                name='vector',
                dimension=dimension,
                index_type=IndexType.HNSW,
                metric_type=MetricType.COSINE,
                params=HNSWParams(m=16, efconstruction=200),
            ))
            index.add(FilterIndex(name='id', field_type=FieldType.String, index_type=IndexType.PRIMARY_KEY))
            index.add(FilterIndex(name='tenant_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='pool_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='type', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='content', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='sensitivity', field_type=FieldType.String, index_type=IndexType.FILTER))

            # 构建 Embedding 配置: 使用 model_name 字符串指定模型
            embedding_cfg = EmbeddingConfig(
                vector_field=EMBEDDING_VECTOR_FIELD,
                field=EMBEDDING_FIELD,
                model_name=EMBEDDING_MODEL_NAME,
            )
            db.create_collection(
                name=coll_name,
                shard=1,
                replicas=1,
                description='EASP Vector Memory Store',
                index=index,
                embedding=embedding_cfg if use_embedding else None,
            )
            self._send_json({
                "code": 0,
                "message": f"Collection {coll_name} created",
                "embedding": "bge-large-zh-v1.5" if use_embedding else None,
                "dimension": EMBEDDING_DIMENSION if use_embedding else dimension,
            })
        except Exception as e:
            if "exist" in str(e).lower():
                self._send_json({"code": 0, "message": f"Collection {coll_name} already exists"})
            else:
                raise

    # ========== Document操作 ==========
    def _handle_insert(self, body):
        """
        插入文档 - 支持两种模式:
        1. 文本模式(推荐): 传 text 字段，向量数据库自动 Embedding
        2. 向量模式(兼容): 传 vector 字段，直接写入向量
        """
        db_name = body.get('database', DEFAULT_DATABASE)
        coll_name = body.get('collection', DEFAULT_COLLECTION)
        documents = body.get('documents', [])

        if not documents:
            self._send_json({"error": "No documents provided"}, 400)
            return

        c = get_client()
        db = c.database(db_name)
        coll = db.collection(coll_name)

        docs = []
        for doc in documents:
            # 如果有 text 字段，使用 Embedding 模式（向量数据库自动向量化）
            if 'text' in doc or 'content' in doc:
                text_content = doc.get('text', doc.get('content', ''))
                kwargs = {}
                if 'id' in doc:
                    kwargs['id'] = doc['id']
                # 设置文本字段 - 向量数据库会自动 Embedding
                kwargs[EMBEDDING_FIELD] = text_content
                # 设置其他字段：兼容两种格式
                # 1) Go InsertText 会把 fields 扁平展开到 doc 顶层
                # 2) 旧接口可能把业务字段放在 doc['fields'] 内
                reserved_keys = {'id', 'text', 'content', 'vector', 'fields'}
                for k, v in doc.items():
                    if k not in reserved_keys:
                        kwargs[k] = v
                if 'fields' in doc:
                    for k, v in doc['fields'].items():
                        if k != EMBEDDING_FIELD:  # 不重复设置 content
                            kwargs[k] = v
                d = Document(**kwargs)
            else:
                # 兼容旧模式：直接传向量
                d = Document(id=doc['id'], vector=doc.get('vector', []))
                if 'fields' in doc:
                    for k, v in doc['fields'].items():
                        setattr(d, k, v)
            docs.append(d)

        start = time.time()
        result = coll.upsert(docs)
        log_event("info", "documents inserted", database=db_name, collection=coll_name, count=len(docs), duration_ms=int((time.time() - start) * 1000))
        self._send_json({"code": 0, "message": "Documents inserted", "count": len(docs), "result": str(result)})

    def _handle_search(self, body):
        """
        搜索文档 - 支持两种模式:
        1. 文本模式(推荐): 传 query_text，向量数据库自动 Embedding 后搜索
        2. 向量模式(兼容): 传 vector，直接用向量搜索
        """
        db_name = body.get('database', DEFAULT_DATABASE)
        coll_name = body.get('collection', DEFAULT_COLLECTION)
        vector = body.get('vector', [])
        query_text = body.get('query_text', '')  # 新增：直接传文本搜索
        limit = body.get('limit', 10)
        filter_str = body.get('filter', '')
        tenant_id = body.get('tenant_id', '')
        pool_id = body.get('pool_id', '')

        c = get_client()
        db = c.database(db_name)
        coll = db.collection(coll_name)

        # 构建 filter 条件
        filters = []
        if filter_str:
            filters.append(filter_str)
        if tenant_id:
            filters.append(f'tenant_id = "{tenant_id}"')
        if pool_id:
            filters.append(f'pool_id = "{pool_id}"')
        combined_filter = ' and '.join(filters) if filters else ''

        search_params = {"limit": limit}
        if combined_filter:
            search_params["filter"] = combined_filter

        # 根据输入类型选择搜索方式
        search_start = time.time()
        if query_text:
            # 文本搜索：向量数据库自动 Embedding
            # RPC SDK 文本搜索方法名是 searchByText，不是 search(embeddingItems=...)
            results = coll.searchByText(embeddingItems=[query_text], **search_params)
        elif vector:
            # 向量搜索：兼容旧模式
            results = coll.search(vectors=[vector], **search_params)
        else:
            self._send_json({"error": "No query_text or vector provided"}, 400)
            return

        search_results = []
        # SDK 返回兼容：
        # - search(vectors=...) 返回 List[List[Dict]]
        # - searchByText(...) 返回 {"documents": List[List[Dict]], "warning": "..."}
        if isinstance(results, dict):
            result_groups = results.get('documents', [])
        else:
            result_groups = results or []

        if result_groups and len(result_groups) > 0:
            for doc in result_groups[0]:
                result_item = {
                    "id": doc.get('id', ''),
                    "score": doc.get('score', 0),
                    "fields": {}
                }
                for k, v in doc.items():
                    if k not in ('id', 'score', 'vector'):
                        result_item["fields"][k] = v
                search_results.append(result_item)

        log_event("info", "documents searched", database=db_name, collection=coll_name, mode="text" if query_text else "vector", limit=limit, result_count=len(search_results), duration_ms=int((time.time() - search_start) * 1000))
        self._send_json({"code": 0, "data": search_results})

    def _handle_delete(self, body):
        db_name = body.get('database', DEFAULT_DATABASE)
        coll_name = body.get('collection', DEFAULT_COLLECTION)
        ids = body.get('ids', [])

        if not ids:
            self._send_json({"error": "No ids provided"}, 400)
            return

        c = get_client()
        db = c.database(db_name)
        coll = db.collection(coll_name)
        start = time.time()
        coll.delete(ids=ids)
        log_event("info", "documents deleted", database=db_name, collection=coll_name, count=len(ids), duration_ms=int((time.time() - start) * 1000))
        self._send_json({"code": 0, "message": "Documents deleted", "count": len(ids)})

    # ========== Embedding操作 ==========
    def _handle_embedding(self, body):
        """
        Embedding 接口 - 保留兼容性
        注意：使用腾讯云内置 Embedding 后，建议直接走 insert/search 接口传文本，
        无需单独获取向量。此接口保留用于特殊场景。
        """
        texts = body.get('texts', [])
        if not texts:
            self._send_json({"error": "No texts provided"}, 400)
            return

        c = get_client()
        db = c.database(DEFAULT_DATABASE)
        coll = db.collection(DEFAULT_COLLECTION)

        try:
            # 使用向量数据库内置 Embedding 获取向量
            # 通过 /document/search 的 embeddingItems 功能间接获取
            # 或者直接调用 tcvectordb SDK 的 embedding 接口
            from tcvectordb.model.document import Document
            # 先 upsert 临时文档获取 embedding，再删除 - 不好
            # 更优方案：直接用 SDK 的 embedding 能力
            embeddings = coll.search(embeddingItems=texts, limit=1)
            # 取出向量
            if embeddings and len(embeddings) > 0 and len(embeddings[0]) > 0:
                doc = embeddings[0][0]
                vec = doc.get('vector', [])
                self._send_json({"code": 0, "data": [vec] * len(texts), "dimension": len(vec) if vec else EMBEDDING_DIMENSION})
            else:
                self._send_json({"code": 0, "data": [], "dimension": EMBEDDING_DIMENSION})
        except Exception as e:
            # 降级：返回维度信息，但告知调用方应直接传文本
            self._send_json({
                "code": 1,
                "message": "推荐直接传文本给 insert/search 接口，由向量数据库自动 Embedding",
                "dimension": EMBEDDING_DIMENSION,
                "error": str(e),
            })


# ========== 启动服务 ==========
def main():
    try:
        c = get_client()
        log_event("info", "connected to vectordb", url=VECTORDB_URL, database=DEFAULT_DATABASE, collection=DEFAULT_COLLECTION)

        # 确保数据库存在
        try:
            c.create_database(DEFAULT_DATABASE)
            log_event("info", "database created", database=DEFAULT_DATABASE)
        except Exception as e:
            log_event("info", "database check", database=DEFAULT_DATABASE, detail=str(e))

        # 确保Collection存在(带 Embedding 配置)
        try:
            db = c.database(DEFAULT_DATABASE)
            index = Index()
            index.add(VectorIndex(
                name=EMBEDDING_VECTOR_FIELD,
                dimension=EMBEDDING_DIMENSION,
                index_type=IndexType.HNSW,
                metric_type=MetricType.COSINE,
                params=HNSWParams(m=16, efconstruction=200),
            ))
            index.add(FilterIndex(name='id', field_type=FieldType.String, index_type=IndexType.PRIMARY_KEY))
            index.add(FilterIndex(name='tenant_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='pool_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='type', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='content', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='sensitivity', field_type=FieldType.String, index_type=IndexType.FILTER))

            embedding_cfg = EmbeddingConfig(
                vector_field=EMBEDDING_VECTOR_FIELD,
                field=EMBEDDING_FIELD,
                model_name=EMBEDDING_MODEL_NAME,
            )
            db.create_collection(
                name=DEFAULT_COLLECTION,
                shard=1,
                replicas=1,
                description='EASP Vector Memory Store',
                index=index,
                embedding=embedding_cfg,
            )
            log_event("info", "collection created", collection=DEFAULT_COLLECTION, embedding=EMBEDDING_MODEL_NAME, dimension=EMBEDDING_DIMENSION)
        except Exception as e:
            log_event("info", "collection check", collection=DEFAULT_COLLECTION, detail=str(e))

    except Exception as e:
        log_event("error", "failed to initialize vectordb", error=str(e), traceback=traceback.format_exc())

    server = HTTPServer(('0.0.0.0', PORT), VectorDBHandler)
    log_event("info", "vector bridge started", port=PORT, health=f"http://localhost:{PORT}/health", embedding=EMBEDDING_MODEL_NAME, dimension=EMBEDDING_DIMENSION)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        log_event("info", "vector bridge shutting down")
        server.shutdown()

if __name__ == '__main__':
    main()
