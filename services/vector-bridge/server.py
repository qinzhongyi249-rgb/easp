#!/usr/bin/env python3
"""
EASP VectorDB Bridge Service
腾讯云向量数据库 HTTP 桥接服务
Port: 8083
"""

import hashlib
import json
import math
import time
import traceback
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse

import tcvectordb
from tcvectordb.model.enum import ReadConsistency, FieldType, IndexType, MetricType
from tcvectordb.model.index import Index, VectorIndex, FilterIndex, HNSWParams
from tcvectordb.model.document import Document

# ========== 配置 ==========
VECTORDB_URL = "http://bj-vdb-c07b6009.sql.tencentcdb.com:8100"
VECTORDB_USER = "root"
VECTORDB_KEY = "kQdmP88Qm6KoqowpjdY9RgKx6VcXG4JjvTDOzFDY"
DEFAULT_DATABASE = "easp_memory"
DEFAULT_COLLECTION = "memories"
EMBEDDING_DIMENSION = 1536
PORT = 8083

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

# ========== 简易Embedding ==========
def text_to_embedding(text, dimension=1536):
    """简易文本转向量 (用于无外部Embedding API时的降级方案)"""
    vectors = []
    for i in range(dimension):
        seed = f"{text}_{i}"
        h = hashlib.sha256(seed.encode()).hexdigest()
        val = int(h[:8], 16) / 0xFFFFFFFF
        val = val * 2 - 1
        vectors.append(val)
    norm = math.sqrt(sum(v * v for v in vectors))
    if norm > 0:
        vectors = [v / norm for v in vectors]
    return vectors

# ========== HTTP处理器 ==========
class VectorDBHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] {args[0]}")

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
            self._send_json({"status": "healthy", "service": "vector-bridge"})
        else:
            self._send_json({"error": "Not found"}, 404)

    def do_OPTIONS(self):
        self._handle_options()

    def do_POST(self):
        path = urlparse(self.path).path
        try:
            body = self._read_body()
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
            traceback.print_exc()
            self._send_json({"error": str(e)}, 500)

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
        dimension = body.get('dimension', EMBEDDING_DIMENSION)
        c = get_client()
        db = c.database(db_name)
        try:
            index = Index()
            index.add(VectorIndex(name='vector', dimension=dimension, index_type=IndexType.HNSW, metric_type=MetricType.COSINE, params=HNSWParams(m=16, efconstruction=200)))
            index.add(FilterIndex(name='id', field_type=FieldType.String, index_type=IndexType.PRIMARY_KEY))
            index.add(FilterIndex(name='tenant_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='pool_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='type', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='content', field_type=FieldType.String, index_type=IndexType.FILTER))

            db.create_collection(
                name=coll_name,
                shard=1,
                replicas=1,
                description='EASP Vector Memory Store',
                index=index,
            )
            self._send_json({"code": 0, "message": f"Collection {coll_name} created"})
        except Exception as e:
            if "exist" in str(e).lower():
                self._send_json({"code": 0, "message": f"Collection {coll_name} already exists"})
            else:
                raise

    # ========== Document操作 ==========
    def _handle_insert(self, body):
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
            d = Document(id=doc['id'], vector=doc['vector'])
            if 'fields' in doc:
                for k, v in doc['fields'].items():
                    setattr(d, k, v)
            docs.append(d)

        result = coll.upsert(docs)
        self._send_json({"code": 0, "message": "Documents inserted", "count": len(docs), "result": result})

    def _handle_search(self, body):
        db_name = body.get('database', DEFAULT_DATABASE)
        coll_name = body.get('collection', DEFAULT_COLLECTION)
        vector = body.get('vector', [])
        limit = body.get('limit', 10)
        filter_str = body.get('filter', '')

        if not vector:
            self._send_json({"error": "No vector provided"}, 400)
            return

        c = get_client()
        db = c.database(db_name)
        coll = db.collection(coll_name)

        search_params = {"limit": limit}
        if filter_str:
            search_params["filter"] = filter_str

        results = coll.search(vectors=[vector], **search_params)

        search_results = []
        if results and len(results) > 0:
            for doc in results[0]:
                result_item = {
                    "id": doc.get('id', ''),
                    "score": doc.get('score', 0),
                    "fields": {}
                }
                for k, v in doc.items():
                    if k not in ('id', 'score', 'vector'):
                        result_item["fields"][k] = v
                search_results.append(result_item)

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
        coll.delete(ids=ids)
        self._send_json({"code": 0, "message": "Documents deleted", "count": len(ids)})

    # ========== Embedding操作 ==========
    def _handle_embedding(self, body):
        texts = body.get('texts', [])
        if not texts:
            self._send_json({"error": "No texts provided"}, 400)
            return
        vectors = [text_to_embedding(text) for text in texts]
        self._send_json({"code": 0, "data": vectors, "dimension": EMBEDDING_DIMENSION})


# ========== 启动服务 ==========
def main():
    try:
        c = get_client()
        print(f"Connected to VectorDB at {VECTORDB_URL}")

        # 确保数据库存在
        try:
            c.create_database(DEFAULT_DATABASE)
            print(f"Created database: {DEFAULT_DATABASE}")
        except Exception as e:
            print(f"Database {DEFAULT_DATABASE} check: {e}")

        # 确保Collection存在
        try:
            db = c.database(DEFAULT_DATABASE)
            index = Index()
            index.add(VectorIndex(name='vector', dimension=EMBEDDING_DIMENSION, index_type=IndexType.HNSW, metric_type=MetricType.COSINE, params=HNSWParams(m=16, efconstruction=200)))
            index.add(FilterIndex(name='id', field_type=FieldType.String, index_type=IndexType.PRIMARY_KEY))
            index.add(FilterIndex(name='tenant_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='pool_id', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='type', field_type=FieldType.String, index_type=IndexType.FILTER))
            index.add(FilterIndex(name='content', field_type=FieldType.String, index_type=IndexType.FILTER))
            db.create_collection(name=DEFAULT_COLLECTION, shard=1, replicas=1, description='EASP Vector Memory Store', index=index)
            print(f"Created collection: {DEFAULT_COLLECTION}")
        except Exception as e:
            print(f"Collection {DEFAULT_COLLECTION} check: {e}")

    except Exception as e:
        print(f"Warning: Failed to initialize VectorDB: {e}")

    server = HTTPServer(('0.0.0.0', PORT), VectorDBHandler)
    print(f"VectorDB Bridge Service running on port {PORT}")
    print(f"  Health: http://localhost:{PORT}/health")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down...")
        server.shutdown()

if __name__ == '__main__':
    main()
