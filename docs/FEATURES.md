# EASP Platform - 项目功能清单

> 最后更新: 2026-06-09

## 一、已完成功能

### 1. 基础设施 ✅

| 功能 | 状态 | 说明 |
|------|------|------|
| Go后端框架 | ✅ | Gin + SQLX |
| MySQL数据库 | ✅ | 阿里云RDS |
| 数据库连接池 | ✅ | 最大25连接 |
| 服务管理脚本 | ✅ | easp.sh |
| nginx反向代理 | ✅ | 8000→8082 |
| Vite前端构建 | ✅ | React 19 + TypeScript + Ant Design 5 |

### 2. 多租户管理 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建租户 | POST | /api/v1/tenants | 创建新租户 (admin_email+admin_pass) |
| 列出租户 | GET | /api/v1/tenants | 获取所有租户 |
| 获取租户 | GET | /api/v1/tenants/:id | 获取单个租户 |
| 更新租户 | PUT | /api/v1/tenants/:id | 更新租户信息 |
| 删除租户 | DELETE | /api/v1/tenants/:id | 删除租户 |

**增强功能**:
- ✅ 租户到期管理 (expires_at, NULL=永久)
- ✅ 用户上限控制 (max_users, 0=不限)
- ✅ 登录/注册校验租户状态+到期+用户上限

### 3. 用户管理 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建用户 | POST | /api/v1/tenants/:tenantId/users | 创建用户 |
| 列出用户 | GET | /api/v1/tenants/:tenantId/users | 列出租户用户 |
| 获取用户 | GET | /api/v1/tenants/:tenantId/users/:id | 获取用户详情 |
| 更新用户 | PUT | /api/v1/tenants/:tenantId/users/:id | 更新用户 |
| 删除用户 | DELETE | /api/v1/tenants/:tenantId/users/:id | 逻辑删除 (deleted_at) |
| 恢复用户 | POST | /api/v1/tenants/:tenantId/users/:id/restore | 恢复已删除用户 |
| 获取角色 | GET | /api/v1/tenants/:tenantId/users/:id/roles | 获取用户角色 |

**增强功能**:
- ✅ 逻辑删除 (deleted_at, NULL=未删)
- ✅ 恢复接口

### 4. 认证系统 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 用户注册 | POST | /api/v1/auth/register | 邮箱/密码注册 |
| 用户登录 | POST | /api/v1/auth/login | 邮箱/密码登录 |
| Token刷新 | POST | /api/v1/auth/refresh | JWT刷新 |
| 获取当前用户 | GET | /api/v1/me | 当前登录用户信息 |
| 修改密码 | PUT | /api/v1/me/password | 修改密码 |
| 获取权限 | GET | /api/v1/me/permissions | 当前用户权限列表 |

**安全特性**:
- ✅ JWT认证中间件
- ✅ 租户状态校验 (注册/登录时)
- ✅ 租户到期校验
- ✅ 用户上限校验

### 5. SSO单点登录 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 租户登录 | POST | /api/v1/sso/:tenantId/login | SSO+标准登录自动回退 |
| 获取SSO配置 | GET | /api/v1/tenants/:tenantId/sso/config | 获取SSO配置 |
| 保存SSO配置 | PUT | /api/v1/tenants/:tenantId/sso/config | 保存SSO配置 |
| 生成登录URL | GET | /api/v1/tenants/:tenantId/sso/login-url | 生成SSO登录链接 |
| 测试连接 | POST | /api/v1/tenants/:tenantId/sso/test | 测试SSO连接 |

**前端页面**:
- ✅ SSO配置页 (/sso-config)
- ✅ 租户登录页 (/sso/:tenantId) - 支持登录+注册双tab (注册tab租户ID锁定)

**数据库表**:
- ✅ tenant_sso_configs (AutoMigrate)

### 6. 角色权限管理 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 列出系统角色 | GET | /api/v1/system/roles | 系统级角色列表 |
| 列出租户角色 | GET | /api/v1/tenants/:tenantId/roles | 租户级角色列表 |
| 创建角色 | POST | /api/v1/tenants/:tenantId/roles | 创建角色 |
| 获取角色 | GET | /api/v1/tenants/:tenantId/roles/:roleId | 获取角色详情 |
| 更新角色 | PUT | /api/v1/tenants/:tenantId/roles/:roleId | 更新角色 |
| 删除角色 | DELETE | /api/v1/tenants/:tenantId/roles/:roleId | 删除角色 |
| 获取角色用户 | GET | /api/v1/tenants/:tenantId/roles/:roleId/users | 获取角色下的用户 |
| 分配角色 | POST | /api/v1/users/assign-role | 分配角色给用户 |
| 撤销角色 | DELETE | /api/v1/users/:userId/roles/:roleId | 撤销用户角色 |
| 获取用户角色 | GET | /api/v1/users/:userId/roles | 获取用户的角色 |

**角色体系**:
- ✅ 两层角色 (系统级 is_system=true / 租户级)
- ✅ admin 拥有 sys-admin + 租户管理员
- ✅ 弹窗+复选框批量分配

### 7. 连接器管理 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建连接器 | POST | /api/v1/tenants/:tenantId/connectors | 创建API连接器 |
| 列出连接器 | GET | /api/v1/tenants/:tenantId/connectors | 列出连接器 |
| 获取连接器 | GET | /api/v1/tenants/:tenantId/connectors/:id | 获取详情 |
| 更新连接器 | PUT | /api/v1/tenants/:tenantId/connectors/:id | 更新连接器 |
| 删除连接器 | DELETE | /api/v1/tenants/:tenantId/connectors/:id | 删除连接器 |

### 8. MCP工具管理 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建工具 | POST | /api/v1/tenants/:tenantId/mcp-tools | 创建MCP工具 |
| 列出工具 | GET | /api/v1/tenants/:tenantId/mcp-tools | 列出工具 |
| 获取工具 | GET | /api/v1/tenants/:tenantId/mcp-tools/:id | 获取详情 |
| 更新工具 | PUT | /api/v1/tenants/:tenantId/mcp-tools/:id | 更新工具 |
| 删除工具 | DELETE | /api/v1/tenants/:tenantId/mcp-tools/:id | 删除工具 |
| 切换状态 | PUT | /api/v1/tenants/:tenantId/mcp-tools/:id/enabled | 启用/禁用 |

**MCP协议**:
- ✅ MCP协议解析器 (JSON-RPC 2.0)
- ✅ MCP消息路由
- ✅ MCP会话管理 (SSE连接)
- ✅ API转MCP (OpenAPI规范解析 v2.0/v3.0)
- ✅ RESTful 单接口导入生成 MCP 工具：支持方法/路径/Schema/生命周期/风险/启用配置，默认草稿且未启用；层面3校验路径必须以 `/` 开头、`required` 必须存在于 `properties`、导入后启用只允许 `published`
- ✅ 内置治理 MCP 工具扩展：支持 `curl` 先测试再创建工具、AI 助手内置创建/更新 Skill，默认给租户管理员角色且锁定不可改
- ✅ AI 助手执行体验优化：状态/心跳/工具结果/模型增量输出会刷新活动时间，避免长流程固定超时误杀；系统提示词和上下文更精简，记忆召回并行加载并使用关键词+向量混合召回
- ✅ Skill 并发编排与内置业务流程：SkillEngine 支持 `parallel` 并发步骤和 `required` 缺参追问；内置“创建用户/创建角色/创建MCP工具”流程默认授权管理员且不可编辑/删除；AI 助手会先做权限预检，权限不足时直接提示缺少的中文权限项，不再追问业务必填项；创建用户不收集明文密码；创建MCP工具强制先测试 curl 再创建 draft/disabled 工具
- ✅ 自动生成MCP工具定义
- ✅ 参数映射引擎
- ✅ 响应转换器
- ✅ 生命周期状态：`draft/testing/published/disabled`，兼容旧 `active/archived`
- ✅ 执行模式：`sandbox/dry_run/production`，正式执行仅允许已发布工具

**MCP网关端点**:
| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| SSE连接 | GET | /api/v1/mcp/:tenantId/sse | MCP SSE连接 |
| 消息处理 | POST | /api/v1/mcp/:tenantId/message | MCP消息 |
| 获取MCP信息 | GET | /api/v1/mcp/:tenantId/info | MCP服务信息 |
| 列出MCP工具 | GET | /api/v1/mcp/:tenantId/tools | MCP工具列表 |
| 调用工具 | POST | /api/v1/mcp/:tenantId/tools/:toolId/call | 调用MCP工具 |
| 导入OpenAPI | POST | /api/v1/mcp/:tenantId/import-openapi | 导入OpenAPI规范 |

### 9. Skill管理 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建Skill | POST | /api/v1/tenants/:tenantId/skills | 创建Skill |
| 列出Skill | GET | /api/v1/tenants/:tenantId/skills | 列出Skill |
| 获取Skill | GET | /api/v1/tenants/:tenantId/skills/:id | 获取详情 |
| 更新Skill | PUT | /api/v1/tenants/:tenantId/skills/:id | 更新Skill |
| 删除Skill | DELETE | /api/v1/tenants/:tenantId/skills/:id | 删除Skill |

**Skill执行引擎**:
| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 执行Skill | POST | /api/v1/tenants/:tenantId/skills/:skillId/execute | 执行Skill |
| 获取执行记录 | GET | /api/v1/skill-executions/:executionId | 获取执行详情 |
| 列出执行记录 | GET | /api/v1/tenants/:tenantId/skill-executions | 列出执行历史 |

**生命周期与沙箱**:
- ✅ Skill/MCP 默认 `draft`，AI 与手动测试默认 `sandbox`
- ✅ `production` 只允许 `published`，`disabled` 禁止执行
- ✅ `sandbox/dry_run` 跳过 MCP/HTTP 外部副作用，执行记录写入 `execution_mode`
- ✅ 前端测试弹窗默认沙箱，结果与历史展示执行模式
- 📄 设计说明：`docs/SKILL_MCP_LIFECYCLE.md`

**数据库表**:
- ✅ skill_executions (AutoMigrate)

### 10. 记忆系统 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建记忆池 | POST | /api/v1/tenants/:tenantId/memory-pools | 创建记忆池 |
| 列出记忆池 | GET | /api/v1/tenants/:tenantId/memory-pools | 列出记忆池 |
| 创建条目 | POST | /api/v1/memory-pools/:poolId/entries | 创建记忆条目 |
| 列出条目 | GET | /api/v1/memory-pools/:poolId/entries | 列出条目 |
| 搜索条目 | GET | /api/v1/memory-pools/:poolId/entries/search | 搜索记忆 |

**记忆治理（层面1）**:
- ✅ 写入去重：`tenant_id + user_id + type + content_hash` 唯一指纹，重复内容不再新增，更新 `last_seen_at/access_count`。
- ✅ 自动提取/召回/敏感过滤/审计开关：`memory_settings` 支持租户级与用户级覆盖。
- ✅ 敏感信息过滤：仅作用于持久化、审计、召回展示链路，不处理实时工具调用参数，避免影响 MCP/Skill 调度。
- ✅ 记忆审计：`memory_audit_logs` 记录 created/deduplicated/redacted/blocked_sensitive/vector_indexed/vector_index_failed 等动作。
- ✅ 混合检索预留：长期记忆写入 MySQL 后异步写入向量库，后续召回可按关键词 + 向量语义合并排序。

**记忆治理（层面2）**:
- ✅ 记忆衰减评分：按 `last_accessed_at/last_seen_at/created_at/access_count/type` 计算召回分。
- ✅ 记忆合并策略：高相似无冲突内容合并，冲突内容保留并写审计。
- ✅ 召回排序增强：从单纯 `created_at DESC` 升级为 `keyword_score + recency_score + frequency_score + type_weight`，预留 `vector_score` 合并入口。

**记忆治理（层面3）**:
- ✅ 混合检索融合：将向量召回分与关键词/衰减/频次/类型权重合并排序，向量服务失败时明确降级到关键词召回。
- ✅ 召回解释：为每条召回记忆输出 keyword/vector/recency/frequency/type/final 分数组成，便于调试和前端展示。
- ✅ 记忆容量限制：按租户/用户/类型容量归档低价值记忆，并写入审计。

### 11. 向量记忆 ✅ (基础实现)

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 保存向量记忆 | POST | /api/v1/tenants/:tenantId/vector-memories | 保存向量记忆 |
| 列出向量记忆 | GET | /api/v1/tenants/:tenantId/vector-memories | 列出向量记忆 |
| 搜索向量记忆 | GET | /api/v1/tenants/:tenantId/vector-memories/search | 语义搜索 |
| 删除向量记忆 | DELETE | /api/v1/tenants/:tenantId/vector-memories/:memoryId | 删除向量记忆 |

**技术实现**:
- ✅ VectorMemoryHandler
- ✅ VectorMemoryService
- ⚠️ 依赖外部Bridge服务 (localhost:8083) - 需要部署向量服务

**数据库表**:
- ✅ memory_vectors (AutoMigrate)

### 12. 模型服务 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 聊天 | POST | /api/v1/model/chat | 非流式聊天 |
| 流式聊天 | POST | /api/v1/model/chat/stream | 流式聊天 |
| 执行Skill | POST | /api/v1/model/skill/execute | 调用Skill |
| 执行MCP | POST | /api/v1/model/mcp/execute | 调用MCP工具 |

**模型配置**:
- ✅ 多供应商支持 (model_providers)
- ✅ 多模型配置 (model_configs)
- ✅ 默认模型互斥 (is_default)
- ✅ enabled/display_name 开关+显示名
- ✅ 供应商禁用检测
- ✅ 禁止硬编码fallback

### 13. AI助手对话 ✅ (新增)

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| SSE流式对话 | POST | /api/v1/tenants/:tenantId/chat | AI助手对话 (SSE流式) |

**功能特性**:
- ✅ SSE流式响应
- ✅ Tool Calling (工具调用)
- ✅ 权限过滤工具列表 (根据用户角色)
- ✅ 气泡链路追踪 (TraceTimeline)
- ✅ 阶段计时 (stage_ms) - thinking/plan/tool_calling/generating
- ✅ 实时滚动计时
- ✅ 模型信息展示 (model/display_name/provider)
- ✅ 工具执行结果展示
- ✅ 上下文承接协议：`conversation_id` + 服务端会话历史 + `page_context`
- ✅ `/assistant` 页面与 `FloatingAssistant` 统一请求协议，前端只传事实型页面上下文，后端负责理解和决策

**内置工具**:
- list_users: 获取用户列表
- get_user: 根据邮箱获取用户
- assign_role: 分配角色
- revoke_role: 撤销角色
- list_roles: 列出角色
- list_connectors: 列出连接器
- list_mcp_tools: 列出MCP工具
- get_tenant_info: 获取租户信息
- update_tenant: 更新租户

**前端页面**:
- ✅ AI助手页面 (/assistant)
- ✅ 浮动助手组件 (FloatingAssistant)

### 14. 模型配置管理 ✅ (增强)

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 创建提供商 | POST | /api/v1/tenants/:tenantId/model-providers | 添加模型提供商 |
| 列出提供商 | GET | /api/v1/tenants/:tenantId/model-providers | 列出提供商 |
| 获取提供商 | GET | /api/v1/tenants/:tenantId/model-providers/:id | 获取详情 |
| 更新提供商 | PUT | /api/v1/tenants/:tenantId/model-providers/:id | 更新提供商 |
| 删除提供商 | DELETE | /api/v1/tenants/:tenantId/model-providers/:id | 删除提供商 |
| 创建配置 | POST | /api/v1/tenants/:tenantId/model-configs | 添加模型配置 |
| 列出配置 | GET | /api/v1/tenants/:tenantId/model-configs | 列出配置 |
| 获取配置 | GET | /api/v1/tenants/:tenantId/model-configs/:id | 获取配置详情 |
| 更新配置 | PUT | /api/v1/tenants/:tenantId/model-configs/:id | 更新配置 |
| 删除配置 | DELETE | /api/v1/tenants/:tenantId/model-configs/:id | 删除配置 |
| 设置默认 | PUT | /api/v1/tenants/:tenantId/model-configs/:id/default | 设为默认 |

### 15. 熔断与限流 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 熔断器状态 | GET | /api/v1/admin/circuit-breakers | 获取熔断器状态 |
| 限流器状态 | GET | /api/v1/admin/rate-limiters | 获取限流器状态 |

**实现**:
- ✅ 熔断器状态机 (Closed/Open/HalfOpen)
- ✅ 失败计数器
- ✅ 半开状态恢复
- ✅ 降级策略配置
- ✅ 令牌桶算法
- ✅ 滑动窗口算法
- ✅ 租户级/用户级/工具级限流

### 16. 审计日志 ✅

| API | 方法 | 路径 | 说明 |
|-----|------|------|------|
| 列出日志 | GET | /api/v1/tenants/:tenantId/audit-logs | 查看审计日志 |

### 17. 前端页面 ✅

| 页面 | 路径 | 说明 |
|------|------|------|
| 仪表盘 | / | 主页面 |
| 登录页 | /login | 用户登录 |
| 租户管理 | /tenants | 租户CRUD |
| 用户管理 | /users | 用户CRUD + 逻辑删除/恢复 |
| 角色管理 | /roles | 角色CRUD + 批量分配 |
| 连接器 | /connectors | 连接器管理 |
| MCP工具 | /mcp-tools | MCP工具管理 |
| Skill | /skills | Skill管理 + 执行测试 |
| 记忆管理 | /memory | 记忆系统管理 |
| 模型配置 | /model-config | 模型提供商+配置管理 |
| SSO配置 | /sso-config | SSO配置管理 |
| AI助手 | /assistant | AI对话助手 |
| 用量分析 | /usage-analytics | API调用量、Token消耗和配额统计 |
| 审计日志 | /audit-logs | 查看审计日志 |
| API Key | /api-keys | 嵌入式聊天/API访问密钥管理 |
| 租户登录 | /sso/:tenantId | SSO+标准登录 |

**菜单权限 key（roles.tools JSON数组）**：`users`、`roles`、`connectors`、`mcp-tools`、`skills`、`memory`、`model-config`、`sso-config`、`usage-analytics`、`audit-logs`、`api-keys`。`dashboard`、`assistant`默认登录可见，`tenants`仅系统级超级管理员可见，`*`表示全部菜单权限。

---

## 二、数据库表清单

| 表名 | 说明 | 状态 |
|------|------|------|
| tenants | 租户表 | ✅ |
| users | 用户表 | ✅ |
| roles | 角色表 | ✅ |
| user_roles | 用户角色关联 | ✅ |
| connectors | 连接器表 | ✅ |
| mcp_tools | MCP工具表 | ✅ |
| memory_pools | 记忆池表 | ✅ |
| memory_entries | 记忆条目表 | ✅ |
| memory_vectors | 向量记忆表 | ✅ |
| skills | Skill模板表 | ✅ |
| skill_executions | Skill执行记录表 | ✅ |
| audit_logs | 审计日志表 | ✅ |
| circuit_breaker_states | 熔断器状态表 | ✅ |
| abac_rules | ABAC规则表 | ✅ |
| sso_providers | SSO提供商配置表 | ✅ |
| tenant_sso_configs | 租户SSO配置表 | ✅ |
| model_providers | 模型提供商表 | ✅ |
| model_configs | 模型配置表 | ✅ |
| assistant_conversations | AI助手会话元数据/页面上下文 | ✅ |
| session_memories | AI助手会话消息历史/短期记忆 | ✅ |

---

### 19. 私有化部署文档 ✅

- ✅ 已补充 `docs/PRIVATE_DEPLOYMENT.md`
- ✅ 覆盖部署形态、硬件规格、软件依赖、目录规划、配置文件、Nginx、systemd、数据库、部署步骤、安全要求、备份恢复、发布回滚和验收清单
- ✅ 明确私有化交付前整改项：数据库/JWT Secret 外部化、清理敏感示例、部署模板化、凭据加密存储

## 四、技术架构

```
┌─────────────────────────────────────────────────────────────┐
│                    前端 (Vite + React 19)                     │
│                  nginx :8000 (外网入口)                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ /api/ 反向代理
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Go后端 (Gin + SQLX)                       │
│                      easp-server :8082                       │
├─────────────────────────────────────────────────────────────┤
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ 租户管理  │  │ 用户管理  │  │ 连接器   │  │ MCP工具  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Skill    │  │ 记忆系统  │  │ 模型服务  │  │ 审计日志  │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │ AI助手   │  │ SSO认证  │  │ 角色权限  │                 │
│  └──────────┘  └──────────┘  └──────────┘                 │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      MySQL / RDS                             │
│                      <mysql-host>                            │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│  模型服务        │ │  向量服务        │ │  MCP服务         │
│  mimo-v2.5-pro  │ │  localhost:8083  │ │  SSE/HTTP       │
│  (小米供应商)    │ │  (待部署)        │ │  (内置)          │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

---

## 四、端口分配

| 端口 | 服务 | 说明 |
|------|------|------|
| 8000 | nginx | 前端页面 + API代理 (外网唯一开放) |
| 8082 | easp-server | Go后端API |
| 5173 | vite-dev | Vite开发服务器 |

---

## 五、服务管理命令

```bash
# EASP后端服务
/home/workCode/easp/easp.sh start     # 启动
/home/workCode/easp/easp.sh stop      # 停止
/home/workCode/easp/easp.sh restart   # 重启
/home/workCode/easp/easp.sh status    # 状态
/home/workCode/easp/easp.sh build     # 编译并重启
/home/workCode/easp/easp.sh logs      # 查看日志

# nginx服务
systemctl start nginx
systemctl stop nginx
systemctl restart nginx
systemctl status nginx

# 前端开发
cd /home/workCode/easp/frontend
npm run dev                           # 启动开发服务器
npm run build                         # 构建生产版本
```

---

## 六、默认配置

| 配置项 | 值 | 说明 |
|------|------|------|
| 默认模型 | mimo-v2.5-pro | 小米供应商 |
| 模型Base URL | https://token-plan-sgp.xiaomimimo.com/v1 | 小米MIMO API |
| 默认账号 | <admin-email> / <initial-password> | 系统管理员，首次登录后必须修改 |
| CodeServer密码 | <codeserver-password> | 仅开发环境，如启用需独立设置 |
