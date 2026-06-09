# EASP Platform - 项目配置文档

> 最后更新: 2026-06-02

## 一、项目路径

```
项目根目录: /home/workCode/easp
前端目录: /home/workCode/easp/frontend
```

---

## 二、服务端口配置

| 端口 | 服务 | 说明 |
|------|------|------|
| 8080 | nginx | 前端页面 + API代理 (外网唯一开放) |
| 8082 | easp-server | Go后端API |
| 8091 | game-dev | 游戏开发服务 |

**重要**: 8080端口是外网唯一开放的端口，不要修改！

---

## 三、nginx配置

配置文件: `/etc/nginx/sites-available/easp.conf`

```nginx
server {
    listen 8080;
    server_name localhost;
    root /home/workCode/enterprise-ai-platform/frontend/static;
    index index.html;
    
    location /assets/ {
        alias /home/workCode/enterprise-ai-platform/frontend/assets/;
    }
    
    location / {
        try_files $uri $uri/ /index.html;
    }
    
    location /api/ {
        proxy_pass http://127.0.0.1:8082;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
    
    location /health {
        proxy_pass http://127.0.0.1:8082/health;
    }
}
```

---

## 四、数据库配置

```yaml
host: rm-8vbh4iqcp8534vs5p6o.mysql.zhangbei.rds.aliyuncs.com
port: 3306
user: easp_dev
password: Easp_dev123
database: easp_dev
charset: utf8mb4
```

---

## 五、模型服务配置

```yaml
base_url: https://maas.apigo.ai/v1
api_key: sk-platform-228fe8d21e2a407f3f35ecf5e1ea72ca3adb23f3023432d2
model: claude-opus-4-7
temperature: 1.0
max_tokens: 4096
```

---

## 六、Go模块配置

go.mod:
```go
module github.com/easp-platform/easp

go 1.21

require (
    github.com/gin-gonic/gin v1.12.0
    github.com/go-sql-driver/mysql v1.7.1
    github.com/jmoiron/sqlx v1.3.5
    github.com/google/uuid v1.6.0
)
```

---

## 七、环境变量

```bash
# 服务端口
PORT=8082

# 数据库
DB_HOST=rm-8vbh4iqcp8534vs5p6o.mysql.zhangbei.rds.aliyuncs.com
DB_PORT=3306
DB_USER=easp_dev
DB_PASSWORD=Easp_dev123
DB_NAME=easp_dev

# 模型服务
MODEL_BASE_URL=https://maas.apigo.ai/v1
MODEL_API_KEY=sk-platform-228fe8d21e2a407f3f35ecf5e1ea72ca3adb23f3023432d2
MODEL_NAME=claude-opus-4-7
```

---

## 八、服务管理

### EASP后端服务

```bash
# 脚本路径
/home/workCode/easp/easp.sh

# 常用命令
./easp.sh start     # 启动
./easp.sh stop      # 停止
./easp.sh restart   # 重启
./easp.sh status    # 状态
./easp.sh build     # 编译并重启
./easp.sh logs      # 查看日志
```

### nginx服务

```bash
# 启动/停止/重启
systemctl start nginx
systemctl stop nginx
systemctl restart nginx

# 查看状态
systemctl status nginx

# 测试配置
nginx -t

# 重新加载配置
nginx -s reload
```

---

## 九、日志路径

| 服务 | 日志路径 |
|------|----------|
| easp-server | /tmp/easp-server.log |
| nginx | /var/log/nginx/access.log |
| nginx错误 | /var/log/nginx/error.log |

---

## 十、编译命令

```bash
cd /home/workCode/easp

# 编译
go build -o easp-server ./cmd/server/

# 使用国内代理编译
GOPROXY=https://goproxy.cn,direct go build -o easp-server ./cmd/server/

# 初始化数据库表
go run ./cmd/init-model-tables/

# 检查数据库
go run ./cmd/check-db/
go run ./cmd/check-schema/
go run ./cmd/check-all-schema/
```

---

## 十一、测试命令

```bash
# 健康检查
curl http://localhost:8080/health

# 创建租户
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Content-Type: application/json" \
  -d '{"name":"Test","plan":"basic","status":"active"}'

# 列出租户
curl http://localhost:8080/api/v1/tenants

# 模型聊天
curl -X POST http://localhost:8080/api/v1/model/chat \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"content":"你好"}]}'
```
