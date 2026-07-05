# EASP 容器化部署指南

## 概述

EASP 开源核心提供 Docker 容器化部署方案，支持以下组件：

| 组件 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| MCP Server | `easp-mcp-server` | 9000 | Python MCP 服务，SSE 端点 |
| MCP Test | `easp-mcp-test` | — | Go MCP 测试工具（CI 用） |

---

## 快速开始

### 前置要求

- Docker 24.0+
- Docker Compose v2.20+

### 一键启动

```bash
# 克隆仓库
git clone https://github.com/qinzhongyi249-rgb/easp.git
cd easp

# 启动 MCP Server
docker compose up -d mcp-server

# 验证
curl http://localhost:9000/sse
```

### 指定端口

```bash
MCP_PORT=8080 docker compose up -d
```

---

## 单独构建镜像

### Python MCP Server

```bash
# 构建
docker build -f mcp-server/Dockerfile -t easp-mcp-server .

# 运行
docker run -d \
  --name easp-mcp \
  -p 9000:9000 \
  -v $(pwd)/mcp-server/mcp_config.json:/app/mcp_config.json:ro \
  easp-mcp-server

# 查看日志
docker logs -f easp-mcp

# 停止
docker stop easp-mcp && docker rm easp-mcp
```

### Go MCP 工具

```bash
# 构建
docker build -t easp-mcp-tools .

# 运行 MCP 测试（连接本地 MCP Server）
docker run --rm \
  --network host \
  -e MCP_SERVER_URL=http://localhost:9000/sse \
  easp-mcp-tools
```

---

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` | `9000` | MCP Server 监听端口 |
| `LOG_LEVEL` | `INFO` | 日志级别 (DEBUG/INFO/WARN/ERROR) |
| `MCP_CONFIG_PATH` | `/app/mcp_config.json` | MCP 配置文件路径 |
| `MCP_SERVER_URL` | `http://localhost:9000/sse` | MCP 服务地址（测试工具用） |

---

## 配置 MCP 工具

编辑 `mcp-server/mcp_config.json` 定义你的 MCP 工具：

```json
{
  "servers": {
    "my-api": {
      "transport": "streamable-http",
      "url": "https://api.example.com",
      "headers": {
        "Authorization": "Bearer ${API_KEY}"
      },
      "tools": [
        {
          "name": "query_data",
          "description": "查询业务数据",
          "parameters": {
            "type": "object",
            "properties": {
              "keyword": {
                "type": "string",
                "description": "搜索关键词"
              }
            },
            "required": ["keyword"]
          }
        }
      ]
    }
  }
}
```

修改后重启服务：

```bash
docker compose restart mcp-server
```

---

## 生产部署建议

### 资源限制

```yaml
# docker-compose.yml 追加
services:
  mcp-server:
    deploy:
      resources:
        limits:
          cpus: "2"
          memory: 512M
        reservations:
          cpus: "0.5"
          memory: 256M
```

### 反向代理 (Nginx)

```nginx
server {
    listen 443 ssl;
    server_name mcp.your-domain.com;

    location /sse {
        proxy_pass http://127.0.0.1:9000/sse;
        proxy_http_version 1.1;
        proxy_buffering off;
        proxy_set_header Connection '';
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

### 健康检查

```bash
# 手动检查
curl -f http://localhost:9000/sse

# Docker 内置健康检查（已配置）
docker inspect --format='{{.State.Health.Status}}' easp-mcp-server
```

---

## 常见问题

### 端口被占用

```bash
# 查看占用
lsof -i :9000

# 使用其他端口
MCP_PORT=9001 docker compose up -d
```

### 构建慢

```bash
# 使用 BuildKit 缓存
DOCKER_BUILDKIT=1 docker compose build

# 禁用缓存重建
docker compose build --no-cache
```

### 日志查看

```bash
# 实时日志
docker compose logs -f mcp-server

# 最近 100 行
docker compose logs --tail=100 mcp-server
```

---

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: MCP E2E Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Start MCP Server
        run: docker compose up -d mcp-server
      - name: Wait for healthy
        run: |
          for i in $(seq 1 30); do
            if curl -sf http://localhost:9000/sse; then
              echo "Server ready"
              exit 0
            fi
            sleep 2
          done
          exit 1
      - name: Run MCP Tests
        run: docker compose --profile test run --rm mcp-test
```
