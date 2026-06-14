# EASP Platform - API转MCP平台

> 企业级智能服务平台 (Enterprise AI Service Platform)

## 项目概述

EASP 是一个将企业API转换为MCP (Model Context Protocol) 服务的平台，支持B端SSO登录和权限控制，为企业提供统一的AI能力接入层。

## 核心功能

- 🏢 **多租户管理** - 完整的租户隔离和管理
- 🔌 **连接器管理** - 支持多种API协议接入
- 🛠️ **MCP工具** - API自动转换为MCP工具
- 🧠 **记忆系统** - 多层级记忆存储和检索
- ⚡ **Skill系统** - 可配置的技能模板和执行引擎
- 🤖 **模型服务** - 多厂商模型配置和调用
- 🔐 **权限控制** - RBAC + ABAC 混合权限模型
- 📊 **审计日志** - 完整的操作审计追踪

## 技术栈

- **后端**: Go + Gin + SQLX
- **数据库**: MySQL (阿里云RDS)
- **前端**: 静态HTML + JavaScript
- **模型服务**: OpenAI兼容API

## 快速开始

```bash
# 启动后端服务
/home/workCode/easp/easp.sh start

# 访问前端
http://服务器IP:8080

# 查看API文档
http://服务器IP:8080/api/v1/tenants
```

## 服务管理

```bash
# EASP后端
./easp.sh start|stop|restart|status|build|logs

# nginx
systemctl start|stop|restart nginx
```

## 文档索引

| 文档 | 说明 |
|------|------|
| [FEATURES.md](docs/FEATURES.md) | 功能清单和API列表 |
| [TODO.md](docs/TODO.md) | 待办任务和开发计划 |
| [DATABASE.md](docs/DATABASE.md) | 数据库设计文档 |
| [CONFIG.md](docs/CONFIG.md) | 项目配置文档 |
| [PRIVATE_DEPLOYMENT.md](docs/PRIVATE_DEPLOYMENT.md) | 私有化部署手册：硬件要求、配置文件、部署步骤、运维验收 |

## 端口配置

| 端口 | 服务 | 说明 |
|------|------|------|
| 8080 | nginx | 前端页面 + API代理 (外网入口) |
| 8082 | easp-server | Go后端API |

## 项目结构

```
easp/
├── cmd/
│   ├── server/              # 主服务入口
│   ├── check-db/            # 数据库检查工具
│   ├── check-schema/        # 表结构检查
│   └── init-model-tables/   # 初始化模型表
├── internal/
│   ├── database/            # 数据库连接
│   ├── models/              # 数据模型
│   ├── repositories/        # 数据访问层
│   ├── handlers/            # API处理器
│   └── modelservice/        # 模型服务
├── docs/                    # 项目文档
├── easp.sh                  # 服务管理脚本
└── README.md
```

## API概览

| 模块 | 路径前缀 | 说明 |
|------|----------|------|
| 租户 | /api/v1/tenants | 多租户管理 |
| 用户 | /api/v1/tenants/:id/users | 用户管理 |
| 连接器 | /api/v1/tenants/:id/connectors | API连接器 |
| MCP工具 | /api/v1/tenants/:id/mcp-tools | MCP工具管理 |
| Skill | /api/v1/tenants/:id/skills | Skill管理 |
| 记忆 | /api/v1/tenants/:id/memory-pools | 记忆系统 |
| 模型 | /api/v1/model | 模型服务调用 |
| 配置 | /api/v1/tenants/:id/model-configs | 模型配置 |

## 开发计划

详见 [TODO.md](docs/TODO.md)

- 🔴 P0: 用户认证、权限系统
- 🟡 P1: MCP协议、熔断限流
- 🟢 P2: 向量记忆、Skill引擎
- ⚪ P3: 前端优化、容器化

## License

Private - Internal Use Only
