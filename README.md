# EASP - 企业级 API-to-MCP 智能网关 (开源核心)

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![Python](https://img.shields.io/badge/Python-3.10+-3776AB?logo=python)](https://www.python.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker)](https://docs.docker.com/)

**EASP (Enterprise AI Service Platform)** 开源核心 —— 包含 MCP 协议实现、MCP Server 框架、嵌入式 AI 助手 SDK。

> 🌐 官网：[jindiyun.com](https://www.jindiyun.com) | 商业版：[easp.jindiyun.com](https://easp.jindiyun.com)

---

## 📦 仓库结构

```
easp/
├── internal/
│   ├── mcp/                       # Go MCP 协议实现
│   │   ├── protocol.go            # MCP 协议核心（JSON-RPC 2.0）
│   │   ├── client.go              # MCP Client（SSE + Streamable HTTP）
│   │   ├── server.go              # MCP Server
│   │   ├── proxy.go               # MCP 代理（透传/治理）
│   │   ├── curl_import.go         # cURL → MCP 工具导入
│   │   └── builtin_governance.go  # 内置治理工具
│   └── openapi/
│       └── parser.go              # OpenAPI → MCP 转换
├── cmd/
│   ├── mcp-test/                  # MCP 测试工具（CLI）
│   └── mcp-e2e/                   # MCP 端到端测试
├── mcp-server/                    # Python MCP Server 框架
│   ├── app/
│   │   ├── mcp_server.py          # MCP Server 实现
│   │   ├── main.py                # 入口
│   │   └── config.py              # 配置
│   ├── requirements.txt
│   ├── mcp_config.json            # 示例配置
│   └── start.sh / restart.sh / stop.sh
├── frontend/                      # 嵌入式 AI 助手 SDK
│   ├── public/embed/
│   │   ├── assistant.js           # 编译产物（5 行代码接入）
│   │   └── assistant-frame.html   # iframe 模式
│   └── src/embed/
│       └── assistant-sdk.ts       # TypeScript 源码
├── docs/                          # 文档
├── migrations/                    # 数据库迁移（SSO 示例）
├── Dockerfile                     # Go MCP Tools 容器镜像
├── docker-compose.yml             # 一键部署 MCP Server
├── LICENSE                        # AGPL v3
└── README.md
```

---

## 🚀 快速开始

### Go MCP 协议库

```bash
go get github.com/qinzhongyi249-rgb/easp
```

```go
import "github.com/qinzhongyi249-rgb/easp/internal/mcp"

// 创建 MCP Client
client := mcp.NewClient("http://localhost:9000/sse")
tools, _ := client.ListTools()
```

### Python MCP Server

```bash
cd mcp-server
pip install -r requirements.txt
bash start.sh
# MCP SSE 端点: http://localhost:9000/sse
```

### 嵌入式 AI 助手 SDK

```html
<script src="https://easp.jindiyun.com/embed/assistant.js"></script>
<script>
  EASPAssistant.init({
    appId: "your-app-id",
    baseUrl: "https://easp.jindiyun.com"
  });
</script>
```

---

## 🐳 Docker 部署

```bash
# 启动 MCP Server + 测试工具
docker compose up -d mcp-server

# 运行 MCP 测试
docker compose --profile test run --rm mcp-test

# 验证
curl http://localhost:9000/sse
```

| 服务 | 端口 | 说明 |
|------|------|------|
| `mcp-server` | 9000 | Python MCP Server (SSE) |
| `mcp-test` | - | Go MCP 测试（一次性） |

---

## 📄 License

Copyright © 2026 北京金砥科技有限公司.

本项目采用 [GNU Affero General Public License v3.0](LICENSE)。
商业授权请联系 [jindiyun.com](https://www.jindiyun.com)。

## 社区

扫码加入飞书群：

<img src="assets/feishu-qr.jpg" width="200" alt="飞书交流群">

---

**北京金砥科技有限公司** | 京ICP备2026038568号-1
