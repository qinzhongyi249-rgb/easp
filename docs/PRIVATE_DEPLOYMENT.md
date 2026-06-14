# EASP 私有化部署手册

> 适用版本：当前 EASP Go + React/Vite + MySQL 架构。
> 文档目标：给企业客户/交付团队说明私有化部署所需硬件、软件、配置文件、部署步骤、验证与运维要求。
> 安全要求：本文不记录真实密码、Token、API Key、证书私钥、数据库连接串；所有敏感值统一使用占位符。

---

## 1. 部署形态

### 1.1 当前推荐形态：单机/小集群裸机部署

当前项目主服务为：

```text
Nginx(8080/443)
  ├─ 前端静态文件 frontend/dist
  └─ /api、/health 反向代理到 Go 后端 127.0.0.1:8082

Go 后端 easp-server(8082)
  ├─ REST API
  ├─ AI 助手 SSE
  ├─ MCP 网关与工具执行
  ├─ Skill 引擎
  ├─ 记忆/审计/权限/租户管理
  └─ MySQL

MySQL 8.0+
  └─ EASP 业务库

可选：Vector Bridge(8083)
  └─ 向量检索/Embedding 桥接服务；当前基础功能可不启用，启用后增强语义记忆召回
```

### 1.2 网络入口

| 端口 | 组件 | 是否对外 | 说明 |
|---|---|---:|---|
| 80/443 | Nginx | 是 | 生产推荐 HTTPS；也可用 8080 做内网入口 |
| 8080 | Nginx | 可选 | 当前测试入口：前端 + API 代理 |
| 8082 | easp-server | 否 | Go 后端，仅允许本机或内网 Nginx 访问 |
| 3306 | MySQL | 否 | 仅后端服务器访问 |
| 8083 | Vector Bridge，可选 | 否 | 仅后端服务器访问 |

生产安全建议：公网只开放 443；8082、3306、8083 不直接暴露公网。

---

## 2. 硬件要求

### 2.1 单机最小规格（试用/PoC）

| 资源 | 要求 |
|---|---|
| CPU | 4 核 x86_64 |
| 内存 | 8 GB |
| 系统盘 | 80 GB SSD |
| 数据盘 | 100 GB SSD，按审计日志/会话/记忆增长扩容 |
| 网络 | 内网千兆；如调用外部模型，需要可访问模型供应商 API |
| 并发建议 | 10-30 个活跃用户，轻量 AI 助手/管理操作 |

### 2.2 推荐生产规格（中小企业）

| 资源 | 要求 |
|---|---|
| CPU | 8 核 x86_64 |
| 内存 | 16-32 GB |
| 系统盘 | 100 GB SSD |
| 数据盘 | 300 GB-1 TB SSD，MySQL 独立数据盘 |
| 网络 | 内网千兆；公网 HTTPS 入口 |
| 并发建议 | 50-200 个活跃用户，常规 MCP/Skill/AI 助手使用 |

### 2.3 高可用/较大规模建议

| 组件 | 建议 |
|---|---|
| Nginx | 2 台或云负载均衡 SLB/ELB |
| Go 后端 | 2 台以上无状态实例，前置负载均衡 |
| MySQL | 主从/高可用版，独立数据库节点 |
| 日志 | 独立日志盘，接入 ELK/Loki/云日志 |
| 备份 | MySQL 每日全量 + binlog 增量；配置和证书单独备份 |

### 2.4 GPU 要求

EASP 当前通过 OpenAI 兼容 API 调用外部/内网模型，本身不要求 GPU。
如果客户要求本地大模型私有化推理，则模型服务独立部署，EASP 只需配置模型供应商 `base_url` 和模型名。

---

## 3. 软件依赖

| 软件 | 版本建议 | 说明 |
|---|---|---|
| Linux | Ubuntu 22.04/24.04 LTS 或 CentOS/Rocky 8+ | 推荐 Ubuntu LTS |
| Go | 1.25.x，至少匹配 `go.mod` | 后端编译 |
| Node.js | 22.x LTS+ | 前端构建；Vite 8/TS 6 建议新版本 Node |
| npm | 随 Node 安装 | 前端依赖安装与构建 |
| MySQL | 8.0+ | 业务数据库，字符集 utf8mb4 |
| Nginx | 1.20+ | 静态文件 + API 反向代理 |
| curl | 任意较新版本 | 健康检查/接口验证 |
| git | 可选 | 拉取代码或部署包 |

---

## 4. 目录规划

推荐生产目录：

```text
/opt/easp/
  ├── easp-server              # Go 后端二进制
  ├── easp.sh                  # 服务管理脚本，可选
  ├── frontend/dist/           # 前端静态文件
  ├── config/                  # 环境变量/配置文件，不提交 Git
  │   └── easp.env
  ├── logs/                    # 应用日志
  └── backups/                 # 本地临时备份，可接对象存储

/etc/nginx/conf.d/easp.conf    # Nginx 配置
/etc/systemd/system/easp.service # systemd 服务，推荐生产使用
/var/lib/mysql 或云 RDS          # MySQL 数据
```

当前开发环境路径为 `/home/workCode/easp`。私有化交付建议改为 `/opt/easp`，并同步修改脚本和 Nginx root 路径。

---

## 5. 当前涉及的配置项与配置文件

### 5.1 Go 后端配置

当前代码中已使用/涉及的配置：

| 配置项 | 当前来源 | 私有化建议 | 说明 |
|---|---|---|---|
| `PORT` | 环境变量，默认 8082 | `easp.env` | 后端监听端口 |
| `EASP_LOG_DIR` | 环境变量，默认项目 `logs` | `easp.env` | 应用日志目录 |
| MySQL Host/Port/User/Password/DB | 当前 `cmd/server/main.go` 中存在硬编码 | 必须改为环境变量/配置文件 | 私有化交付前必须外部化 |
| JWT Secret | 当前 `internal/auth/jwt.go` 中存在硬编码 | 必须改为环境变量 | 生产必须每客户独立随机生成 |
| 模型供应商 `base_url/api_key/model` | 数据库 `model_providers/model_configs` | 管理后台配置 | 不应写入部署文档明文 |
| SSO 配置 | 数据库 `tenant_sso_configs` | 管理后台配置 | 每租户独立配置 |
| Connector 下游凭据 | 数据库 `connectors` | 管理后台配置 | 建议加密存储/最小权限 |
| SSL 证书 | Nginx 文件 | `/etc/nginx/ssl/` | 私钥权限 600 |

> 重要：当前 `docs/CONFIG.md` 和部分代码里存在开发环境示例值。私有化交付包必须清理真实敏感值，只保留占位符。

### 5.2 推荐环境变量文件 `/opt/easp/config/easp.env`

```bash
# 服务
PORT=8082
GIN_MODE=release
EASP_LOG_DIR=/opt/easp/logs

# 数据库
DB_HOST=<mysql-host>
DB_PORT=3306
DB_USER=<mysql-user>
DB_PASSWORD=<mysql-password>
DB_NAME=easp

# 安全
JWT_SECRET=<generate-strong-random-secret>
TOKEN_ENCRYPTION_KEY=<generate-32-byte-random-key>

# 可选：向量桥接服务
VECTOR_BRIDGE_URL=http://127.0.0.1:8083
VECTOR_BRIDGE_ENABLED=false

# 可选：外部访问地址，用于回调/文档展示
PUBLIC_BASE_URL=https://easp.example.com
```

生成随机密钥示例：

```bash
openssl rand -base64 48
openssl rand -hex 32
```

### 5.3 Nginx 配置 `/etc/nginx/conf.d/easp.conf`

#### HTTP 内网/测试版

```nginx
server {
    listen 8080;
    server_name _;

    root /opt/easp/frontend/dist;
    index index.html;

    client_max_body_size 20m;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # AI 助手/SSE 流式响应必须关闭缓冲
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    location /health {
        proxy_pass http://127.0.0.1:8082/health;
    }
}
```

#### HTTPS 生产版

```nginx
server {
    listen 80;
    server_name easp.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name easp.example.com;

    ssl_certificate     /etc/nginx/ssl/easp.crt;
    ssl_certificate_key /etc/nginx/ssl/easp.key;
    ssl_protocols TLSv1.2 TLSv1.3;

    root /opt/easp/frontend/dist;
    index index.html;
    client_max_body_size 20m;

    location / {
        try_files $uri $uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 3600s;
        proxy_send_timeout 3600s;
    }

    location /health {
        proxy_pass http://127.0.0.1:8082/health;
    }
}
```

### 5.4 systemd 服务 `/etc/systemd/system/easp.service`

```ini
[Unit]
Description=EASP Platform Backend
After=network.target mysql.service

[Service]
Type=simple
WorkingDirectory=/opt/easp
EnvironmentFile=/opt/easp/config/easp.env
ExecStart=/opt/easp/easp-server
Restart=always
RestartSec=5
User=easp
Group=easp
LimitNOFILE=65535

# 基础安全加固
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=/opt/easp/logs

[Install]
WantedBy=multi-user.target
```

---

## 6. 数据库准备

### 6.1 创建数据库和账号

```sql
CREATE DATABASE easp DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'easp_user'@'%' IDENTIFIED BY '<strong-password>';
GRANT ALL PRIVILEGES ON easp.* TO 'easp_user'@'%';
FLUSH PRIVILEGES;
```

生产建议：

- MySQL 只允许 EASP 后端服务器访问。
- 开启 binlog，便于误操作恢复。
- 定期备份：每日全量 + binlog 增量。
- 数据库账号仅授权 EASP 业务库，不使用 root。

### 6.2 表结构初始化

当前后端启动时会执行 `database.AutoMigrate()` 并创建/补齐表结构。部署后首次启动即可初始化基础表。

验证：

```bash
cd /opt/easp
./easp-server
# 或 systemctl start easp
```

然后检查健康接口和数据库表。

---

## 7. 部署步骤

### 7.1 安装系统依赖

Ubuntu 示例：

```bash
apt update
apt install -y nginx mysql-client curl git build-essential openssl

# 安装 Go 1.25.x：按企业基础镜像或官方包安装
# 安装 Node.js 22.x：按企业基础镜像或 NodeSource/离线包安装
```

生产内网环境建议提前准备离线包：

- Go 安装包
- Node.js 安装包
- npm 依赖缓存或 `node_modules` 构建产物
- EASP 后端二进制
- 前端 `dist` 产物

### 7.2 创建运行用户和目录

```bash
useradd -r -s /usr/sbin/nologin easp || true
mkdir -p /opt/easp/{config,logs,frontend,backups}
chown -R easp:easp /opt/easp
```

### 7.3 构建后端

在构建机或目标机：

```bash
cd /opt/easp-src
GOPROXY=https://goproxy.cn,direct go build -o easp-server ./cmd/server/
```

复制到运行目录：

```bash
cp easp-server /opt/easp/easp-server
chmod +x /opt/easp/easp-server
chown easp:easp /opt/easp/easp-server
```

### 7.4 构建前端

```bash
cd /opt/easp-src/frontend
npm ci
npm run build
rm -rf /opt/easp/frontend/dist
mkdir -p /opt/easp/frontend
cp -r dist /opt/easp/frontend/
chown -R easp:easp /opt/easp/frontend
```

前端 API 当前使用相对路径 `/api/v1`，通常不需要单独配置后端地址，由 Nginx 反向代理统一处理。

### 7.5 写入环境变量

```bash
cat >/opt/easp/config/easp.env <<'EOF'
PORT=8082
GIN_MODE=release
EASP_LOG_DIR=/opt/easp/logs
DB_HOST=<mysql-host>
DB_PORT=3306
DB_USER=<mysql-user>
DB_PASSWORD=<mysql-password>
DB_NAME=easp
JWT_SECRET=<strong-random-secret>
TOKEN_ENCRYPTION_KEY=<strong-random-key>
PUBLIC_BASE_URL=https://easp.example.com
VECTOR_BRIDGE_ENABLED=false
VECTOR_BRIDGE_URL=http://127.0.0.1:8083
EOF

chmod 600 /opt/easp/config/easp.env
chown easp:easp /opt/easp/config/easp.env
```

### 7.6 注册 systemd 服务

```bash
cp easp.service /etc/systemd/system/easp.service
systemctl daemon-reload
systemctl enable easp
systemctl start easp
systemctl status easp --no-pager
```

### 7.7 配置 Nginx

```bash
cp easp.conf /etc/nginx/conf.d/easp.conf
nginx -t
systemctl reload nginx
```

### 7.8 验证部署

```bash
# 后端直连，仅本机
curl -fsS http://127.0.0.1:8082/health

# Nginx 入口
curl -fsS http://127.0.0.1:8080/health
# 或生产域名
curl -fsS https://easp.example.com/health
```

期望返回：

```json
{"service":"EASP Platform","status":"healthy"}
```

---

## 8. 初始化业务配置

### 8.1 首次登录

系统启动后会初始化默认角色、默认管理员和内置治理能力。
私有化交付时必须在首次登录后立即修改默认管理员密码，并创建正式租户管理员账号。

### 8.2 模型供应商配置

路径：`模型配置 /model-config`

需要配置：

| 字段 | 说明 |
|---|---|
| Provider Name | 供应商标识，例如 `local-openai-compatible` |
| Display Name | 页面显示名 |
| Base URL | OpenAI 兼容模型服务地址，例如 `https://model.example.com/v1` |
| API Key | 模型服务访问凭据，不写入文档 |
| Model Name | 模型名称 |
| Default | 每租户只能一个默认模型 |

要求：

- 不允许静默 fallback 到错误模型配置。
- 无可用默认模型时，应明确提示配置错误。
- API Key 只在后台录入，不出现在部署文档和日志中。

### 8.3 SSO 配置，可选

路径：`SSO配置 /sso-config`

支持租户级 SSO。私有化常见接入：

- 企业 IdP 登录接口
- 用户信息接口
- 响应字段映射
- 登录后同步用户

SSO Token 透传是连接器可选能力，不是默认行为。无用户 SSO Token 时必须明确报错，不应降级为固定凭据。

### 8.4 Connector/MCP/Skill 配置

- Connector：配置企业内部 API 或 MCP Server。
- MCP 工具：OpenAPI/REST/curl 导入生成，生产调用只允许 `published + enabled`。
- Skill：AI 创建默认 `draft`，测试默认 `sandbox/dry_run`，生产执行只允许 `published`。
- 内置治理工具和内置 Skill 默认授权管理员且锁定，不应允许误删误改。

---

## 9. 向量服务，可选

当前 EASP 已有基础向量记忆表和 VectorMemoryService，文档中标记外部 Bridge 服务默认地址为 `localhost:8083`。如果客户暂不启用向量服务：

- 普通记忆、关键词召回、AI 助手、MCP、Skill 可继续运行。
- 语义检索能力会降级，不应阻塞主流程。

启用时建议：

```bash
VECTOR_BRIDGE_ENABLED=true
VECTOR_BRIDGE_URL=http://127.0.0.1:8083
```

并将 8083 限制为本机/内网访问。

---

## 10. 安全要求

### 10.1 必改项

私有化上线前必须完成：

1. 数据库连接配置外部化，不能硬编码。
2. JWT Secret 外部化，且每客户独立随机生成。
3. 清理文档/日志/示例中的真实密码、Token、API Key。
4. Nginx 开启 HTTPS。
5. 8082/3306/8083 不暴露公网。
6. 默认管理员密码首次登录后立即修改。
7. 生产证书私钥权限设置为 `600`。
8. 模型 API Key、连接器凭据、SSO Token 不写日志。

### 10.2 文件权限

```bash
chmod 600 /opt/easp/config/easp.env
chmod 700 /opt/easp/config
chmod 755 /opt/easp
chmod -R 750 /opt/easp/logs
```

### 10.3 日志脱敏

当前 logger 已有敏感字段脱敏规则，覆盖：

- authorization
- access_token
- refresh_token
- api_key
- password
- secret
- credential
- cookie
- token/key 等

交付验证时应主动搜索日志，确认没有真实凭据。

---

## 11. 运维命令

### 11.1 systemd 方式

```bash
systemctl status easp --no-pager
systemctl restart easp
journalctl -u easp -f
```

### 11.2 easp.sh 方式，当前项目自带

当前脚本路径：`/home/workCode/easp/easp.sh`。私有化部署到 `/opt/easp` 后需要同步修改脚本中的：

```bash
APP_DIR="/opt/easp"
PID_FILE="/tmp/easp-server.pid"
LOG_DIR="${EASP_LOG_DIR:-$APP_DIR/logs}"
PORT=8082
```

常用命令：

```bash
./easp.sh start
./easp.sh stop
./easp.sh restart
./easp.sh status
./easp.sh build
./easp.sh logs
./easp.sh errors
```

生产更推荐 systemd，`easp.sh` 可作为开发/测试辅助脚本。

---

## 12. 备份与恢复

### 12.1 需要备份的内容

| 内容 | 路径/位置 | 频率 |
|---|---|---|
| MySQL 数据库 | `easp` 库 | 每日全量 + binlog 增量 |
| 环境变量配置 | `/opt/easp/config/easp.env` | 变更后立即备份，需加密 |
| Nginx 配置 | `/etc/nginx/conf.d/easp.conf` | 变更后备份 |
| SSL 证书 | `/etc/nginx/ssl/` | 变更后备份，私钥加密保存 |
| 前端/后端版本包 | `/opt/easp/easp-server`、`frontend/dist` | 每次发布留档 |

### 12.2 MySQL 备份示例

```bash
mysqldump -h <mysql-host> -u <mysql-user> -p --single-transaction --routines --triggers easp \
  | gzip > /opt/easp/backups/easp-$(date +%F).sql.gz
```

### 12.3 恢复演练

至少每季度做一次恢复演练：

1. 新建临时 MySQL 实例。
2. 恢复最近全量备份。
3. 应用 binlog 到指定时间点。
4. 启动 EASP 指向临时库。
5. 验证登录、租户、角色、MCP、Skill、AI 助手。

---

## 13. 发布与回滚

### 13.1 发布前检查

```bash
go test ./...
npm run build --prefix frontend
nginx -t
```

### 13.2 发布步骤

```bash
# 备份当前版本
mkdir -p /opt/easp/backups/release-$(date +%F-%H%M%S)
cp /opt/easp/easp-server /opt/easp/backups/release-$(date +%F-%H%M%S)/
cp -r /opt/easp/frontend/dist /opt/easp/backups/release-$(date +%F-%H%M%S)/dist

# 替换新版本
cp easp-server /opt/easp/easp-server
rm -rf /opt/easp/frontend/dist
cp -r dist /opt/easp/frontend/dist
chown -R easp:easp /opt/easp

# 重启验证
systemctl restart easp
curl -fsS http://127.0.0.1:8082/health
systemctl reload nginx
curl -fsS https://easp.example.com/health
```

### 13.3 回滚

```bash
systemctl stop easp
cp /opt/easp/backups/<release>/easp-server /opt/easp/easp-server
rm -rf /opt/easp/frontend/dist
cp -r /opt/easp/backups/<release>/dist /opt/easp/frontend/dist
systemctl start easp
curl -fsS http://127.0.0.1:8082/health
```

数据库结构升级需要单独评估回滚策略。上线前必须备份数据库。

---

## 14. 私有化交付前代码整改清单

当前代码已能在现有环境运行，但作为客户私有化交付，建议先完成以下整改：

- [ ] `cmd/server/main.go`：数据库 Host/Port/User/Password/DB 改为环境变量读取。
- [ ] `internal/auth/jwt.go`：JWT Secret 改为环境变量读取，禁止打印 secret。
- [ ] `docs/CONFIG.md`：替换开发环境真实连接信息为占位符。
- [ ] `easp.sh`：支持 `APP_DIR`、`PORT`、`EASP_LOG_DIR` 从环境变量覆盖。
- [ ] 提供 `deploy/templates/easp.env.example`、`easp.service`、`nginx.conf` 模板。
- [ ] 确认模型供应商、连接器、SSO Token 等敏感数据加密存储策略。
- [ ] 补充容器化部署方案，如 Docker Compose/Kubernetes，为后续标准交付做准备。

---

## 15. 验收清单

| 检查项 | 命令/方式 | 期望 |
|---|---|---|
| 后端健康 | `curl http://127.0.0.1:8082/health` | healthy |
| Nginx 入口 | `curl https://easp.example.com/health` | healthy |
| 前端访问 | 浏览器访问域名 | 页面正常加载 |
| 登录 | 管理员登录 | 成功进入系统 |
| 租户 | 创建/查看租户 | 正常 |
| 用户/角色 | 创建用户、分配角色 | 正常 |
| 权限菜单 | 角色 tools 配置 | 前端菜单正确显示 |
| MCP | 创建/测试 MCP 工具 | draft/testing/published 流程符合预期 |
| Skill | 执行内置 Skill | 缺参会追问，生产只允许 published |
| AI 助手 | SSE 对话 | 流式返回，不被 Nginx 缓冲 |
| 审计日志 | 查看操作记录 | 有记录且无敏感明文 |
| 备份 | 执行 mysqldump | 可恢复 |

---

## 16. 常见问题

### 16.1 前端能打开但 API 失败

检查：

```bash
curl http://127.0.0.1:8082/health
curl http://127.0.0.1:8080/health
nginx -t
journalctl -u easp -n 100 --no-pager
```

重点看 Nginx `/api/` 是否代理到 `127.0.0.1:8082`。

### 16.2 AI 助手没有流式输出

检查 Nginx：

```nginx
proxy_buffering off;
proxy_cache off;
proxy_read_timeout 3600s;
```

### 16.3 模型调用失败

检查后台模型配置：

- Provider 是否启用。
- Model Config 是否启用。
- 是否设置默认模型。
- `base_url` 是否可从服务器访问。
- API Key 是否有效。

不要把 API Key 打印到日志或粘贴到文档。

### 16.4 数据库连接失败

检查：

```bash
mysql -h <mysql-host> -u <mysql-user> -p -e 'select 1'
```

确认安全组/防火墙允许 EASP 后端访问 MySQL。

---

## 17. 当前环境快速对照

当前开发/测试环境使用：

| 项 | 当前值 |
|---|---|
| 项目路径 | `/home/workCode/easp` |
| 后端端口 | `8082` |
| Nginx 入口 | `8080` |
| 前端构建目录 | `/home/workCode/easp/frontend/dist` |
| 管理脚本 | `/home/workCode/easp/easp.sh` |
| 日志目录 | `/home/workCode/easp/logs` |

私有化部署时建议统一迁移到 `/opt/easp` 并使用 systemd 托管。
