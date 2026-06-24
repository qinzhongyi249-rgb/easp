# EASP Platform - 待办任务清单

> 最后更新: 2026-06-09

## 优先级说明

- 🔴 P0 - 紧急/核心功能
- 🟡 P1 - 重要功能
- 🟢 P2 - 增强功能
- ⚪ P3 - 未来规划

---

## 一、安全与认证 ✅ 已完成

### 1.1 用户认证系统
- [x] JWT认证中间件
- [x] 用户登录API (邮箱/密码)
- [x] 用户注册API
- [x] Token刷新机制
- [ ] 登录失败锁定策略

### 1.2 身份源配置 / SSO 登录 EASP 控制台
- [x] SSO配置管理 (tenant_sso_configs)
- [x] 租户登录API (/sso/:tenantId/login)
- [x] SSO+标准登录自动回退
- [x] 租户登录页 (/sso/:tenantId) 支持登录+注册双tab
- [x] 登录页统一：/login 与 /sso/:tenantId 共用同一登录/注册页面；带租户号访问时自动填充租户号并锁定不可修改，登录仍由后端 /sso/:tenantId/login 自动处理 SSO/标准登录回退
- [x] OAuth2.0集成 (企业微信/钉钉/飞书/自定义)
- [x] 租户级 SSO 配置说明手册公开访问：`/docs/sso.html` 与 `/docs/sso.md`，面向租户管理员和系统集成方，说明前置准备、配置流程、示例、常见问题
- [ ] SAML2.0集成

### 1.3 集成中心产品重构 🟡 P1
- [x] 明确定位：应用接入是主路径，SSO 是员工登录 EASP 控制台的辅助路径，业务工具接入是增强路径；详见 `docs/INTEGRATION_CENTER.md`
- [x] 应用接入页增加「接入指南」与「联调检测」，让业务系统开发者按后端换 Token + 前端嵌入助手完成接入
- [x] 应用接入页升级为接入工作台：流程步骤、统计卡片、应用卡片、步骤化指南、联调 checklist
- [x] 后端提供应用接入 guide/diagnose 接口，返回明确错误码、检查项和修复建议
- [x] SSO 页面改为身份源模板向导，默认隐藏高级 OAuth 字段
- [x] 业务工具接入页升级为接入工作台：MCP/OpenAPI/REST入口、凭据模式说明、统计卡片、接入源卡片、工具管理入口和接入源治理详情抽屉（来源/锁定/传输/凭据模式/工具数/URL/治理建议）
- [x] MCP 工具页升级为工具导入与治理工作台：OpenAPI批量导入、REST单接口、MCP发现、生命周期/授权提示、统计卡片和工具治理详情抽屉（来源/生命周期/启用/锁定/风险/Schema/路径/生产可用提示）
- [x] MCP 工具治理详情补齐实时授权与可执行状态：后端提供 `GET /api/v1/tenants/:tenantId/mcp-tools/:toolId/governance-status`，返回已授权角色数/角色列表、当前用户是否可执行、命中角色和不可执行原因；前端抽屉新增「授权与可执行状态」分区
- [x] 角色管理页升级为授权工作台：菜单/MCP/Skill/数据/限流分区、角色卡片、授权统计和执行链路提示
- [x] 审计日志页升级为审计可观测工作台：来源/外部身份/内部用户/工具/结果/耗时统计、过滤和分区详情
- [x] 用量分析页升级为成本与调用可观测工作台：Token/缓存命中/耗时/来源/工具成功率/明细详情和审计交叉定位提示
- [x] 模型配置页升级为模型路由与健康工作台：默认模型、启用供应商/模型、备用模型、测试连接、fallback风险提示
- [x] 记忆管理页升级为记忆治理工作台：记忆池、来源状态、向量索引率、治理链路、长期记忆详情和治理审计详情
- [x] API Key 管理页升级为凭证安全工作台：用途边界、可用凭证、高风险、从未使用、权限 scopes 覆盖率、轮换建议和安全详情
- [x] 用户管理页升级为租户用户运营工作台：租户用户、活跃/注销、需关注账号、角色授权、身份映射、软删除恢复和用户详情风险提示
- [x] 租户管理页升级为租户运营与容量工作台：租户总数、正常租户、到期风险、容量风险、用户容量、生命周期、配额治理、租户详情和创建管理员提示
- [x] Skill 管理页升级为 Skill 治理工作台：来源、内置锁定、生命周期、生产可用、Schema/触发、执行编排、详情/测试和历史可见
- [x] 内置/锁定资源后端保护：Skill(created_by=system)、MCP Tool(is_builtin/locked)、Connector(is_builtin/locked/type=builtin) 不可删除、停用或普通编辑；API 与助手治理工具均强制拦截

### 1.4 API安全
- [x] 将 `cmd/server/main.go` 中硬编码数据库连接信息迁移到环境变量/配置文件，并更新 systemd EnvironmentFile，避免源码和排障输出暴露凭据
- [ ] API Key认证
- [ ] 请求签名验证
- [ ] IP白名单
- [ ] HTTPS强制

---

## 二、权限系统 ✅ 已完成

### 2.1 RBAC基础权限
- [x] 角色CRUD API
- [x] 权限分配API
- [x] 两层角色体系 (系统级/租户级)
- [x] 弹窗+复选框批量分配
- [ ] 角色继承机制

### 2.2 ABAC高级权限
- [ ] ABAC规则引擎实现
- [ ] 规则条件解析器
- [ ] 规则优先级处理
- [ ] 规则测试工具

### 2.3 数据权限
- [x] 租户数据隔离
- [ ] 部门数据权限
- [ ] 个人数据权限
- [ ] 自定义数据范围

---

## 三、MCP协议实现 ✅ 已完成

### 3.1 MCP服务器
- [x] MCP协议解析器 (JSON-RPC 2.0)
- [x] MCP消息路由
- [x] MCP会话管理 (SSE连接)
- [x] MCP错误处理

### 3.2 API转MCP
- [x] OpenAPI规范解析 (v2.0/v3.0)
- [x] 自动生成MCP工具定义
- [x] 参数映射引擎
- [x] 响应转换器
- [x] RESTful API 单接口导入生成 MCP 工具：支持 GET/POST/PUT/PATCH/DELETE 方法、路径、参数 Schema 与状态配置；层面2已补充 input_schema 严格 JSON 校验、生命周期/风险/启用配置，默认草稿且未启用，避免直接暴露生产调用；层面3已补充路径必须以 `/` 开头、required 必须存在于 properties、导入后启用只允许 published
- [x] 内置治理 MCP 工具扩展：支持 `curl` 导入先测试再创建 MCP 工具，AI 助手可复用；支持 AI 助手内置创建/更新 Skill；默认授权租户管理员角色且不可取消/删除/停用/编辑
- [x] AI 助手超时与理解效率优化：状态/心跳/工具结果/模型增量输出刷新活动时间；连续工具调用上限从 5 轮提升到 8 轮并明确提示“步骤上限”；精简系统提示词、限制历史与技能上下文；记忆召回并行加载并继续使用关键词+向量混合召回
- [x] Skill 编排能力增强：支持 `parallel` 并发步骤和 `required` 缺参追问；新增内置创建用户/创建角色/创建MCP工具流程 Skill，管理员默认授权且后端保留权限；内置 Skill 不可编辑/删除；AI 助手先做权限预检，权限不足时提示缺少的中文权限项，不再追问业务必填项；缺参追问字段使用中文口语化标签；创建用户流程不收集明文密码；创建MCP工具流程强制先测试 curl 再基于测试结果创建 draft/disabled 工具
- [ ] 连接器用户身份透传：支持将当前 SSO 登录用户 Token 动态注入下游 REST/MCP 调用 Header，标准登录无 Token 时明确报错

### 3.3 MCP工具调用
- [x] 工具调用链路
- [x] 参数验证
- [x] 结果缓存
- [x] 调用超时控制

---

## 四、熔断与限流 ✅ 已完成

### 4.1 熔断器
- [x] 熔断器状态机实现 (Closed/Open/HalfOpen)
- [x] 失败计数器
- [x] 半开状态恢复
- [x] 降级策略配置

### 4.2 限流器
- [x] 令牌桶算法实现
- [x] 滑动窗口算法实现
- [x] 租户级限流
- [x] 用户级限流
- [x] 工具级限流

### 4.3 重试机制
- [ ] 指数退避重试
- [ ] 重试次数限制
- [ ] 重试条件判断
- [ ] 幂等性保证

---

## 五、AI助手对话 ✅ 已完成 (新增)

### 5.1 对话功能
- [x] SSE流式响应
- [x] Tool Calling (工具调用)
- [x] 权限过滤工具列表
- [x] 系统提示词生成
- [x] 上下文承接协议：`conversation_id`、服务端会话历史、`page_context`
- [x] `/assistant` 与 `FloatingAssistant` 统一请求协议

### 5.2 链路追踪
- [x] 气泡链路追踪 (TraceTimeline)
- [x] 阶段计时 (stage_ms) - thinking/plan/tool_calling/generating
- [x] 实时滚动计时
- [x] 模型信息展示

### 5.3 内置工具
- [x] 用户管理工具 (list_users, get_user)
- [x] 角色管理工具 (assign_role, revoke_role, list_roles)
- [x] 连接器工具 (list_connectors)
- [x] MCP工具 (list_mcp_tools)
- [x] 租户工具 (get_tenant_info, update_tenant)

### 5.4 前端集成
- [x] AI助手页面 (/assistant)
- [x] 浮动助手组件 (FloatingAssistant)

### 5.5 嵌入式 AI 助手 🟡 P1
- [x] 嵌入式 AI 助手设计文档：`docs/EMBEDDED_ASSISTANT.md`
- [x] 接入应用配置表 `tenant_embed_apps`：app_id/app_secret_hash/allowed_origins/token_ttl/status 等
- [x] 外部用户绑定表 `external_user_bindings`：优先外部用户导入，不默认自动创建用户
- [x] 外部用户导入 API：支持批量导入、冲突明确返回、角色分配、profile/attributes/metadata 与第三方 identities
- [x] 外部用户映射查询/管理 API：支持 external_system/status/keyword/limit 基础查询
- [x] 嵌入式 Token 换取 API：`/api/v1/embed/token/exchange`，返回 `easp-api-token`
- [x] `easp-api-token` 鉴权中间件：仅允许嵌入式助手白名单接口
- [x] 现有 AI 助手聊天接口支持 embed token，并复用 EASP 用户/角色/工具/Skill/MCP 权限规则
- [x] JS SDK 最小入口：`/embed/assistant.js`
- [x] iframe 最小入口：`/embed/assistant-frame.html`
- [x] 嵌入式历史会话查询接口：业务系统按文档独立对接
- [x] 第三方接入手册公开访问：`/docs/embedded-assistant.html` 与 `/docs/embedded-assistant.md`，内容面向第三方业务系统开发者，聚焦其需要实现的服务端换 Token、页面嵌入和外部用户身份传递，避免混入 EASP 内部实现任务
- [x] 用户体系扩展：`users.user_uid/profile/attributes` 与 `user_identity_bindings`，支持微信/飞书等第三方身份关联信息维护（暂不做 OAuth 对接）
- [x] 管理后台页面：用户管理页新增「平台用户 / 外部用户 / 第三方身份 / 嵌入接入应用」四个 Tab，支持基础查询、导入外部用户、查看接入手册、新建接入应用并一次性展示 App Secret
- [x] 审计增强：`audit_logs` 固化当次访问来源快照（source_type/source_app_id/external_system/external_user_id/user_uid），审计页支持来源、接入应用、外部系统、外部用户、EASP用户过滤与展示

---

## 六、模型服务增强 🟡 P1

### 6.1 多模型支持
- [x] 模型提供商管理 (model_providers)
- [x] 模型配置管理 (model_configs)
- [x] 默认模型互斥设置
- [x] enabled/display_name 开关+显示名
- [x] 供应商禁用检测
- [ ] OpenAI GPT系列
- [ ] 文心一言
- [ ] 通义千问
- [ ] 智谱GLM
- [ ] 自定义模型接入

### 6.2 模型路由
- [ ] 按任务类型路由
- [ ] 按成本路由
- [ ] 按延迟路由
- [ ] 负载均衡

### 6.3 模型监控
- [x] 基础 Token 消耗记录 (model_usage)
- [x] 基础 API 调用记录 (api_usage)
- [ ] 用量分析独立页面（年月日汇总+明细+图表）
- [ ] 功能来源统计（AI助手/MCP/Skill/Embed）
- [ ] MCP工具/Skill调用次数统计
- [ ] 响应时间监控
- [ ] 错误率监控

> 设计文档：`docs/USAGE_ANALYTICS.md`

---

## 七、向量记忆系统 🟢 P2 (重点推进)

### 7.1 向量存储基础设施
- [x] 向量记忆表 (memory_vectors)
- [x] VectorMemoryHandler (CRUD API)
- [x] VectorMemoryService (基础实现)
- [ ] **部署向量服务** (替代 localhost:8083 Bridge)
- [ ] 选择向量数据库 (Milvus/Qdrant/Chroma)
- [ ] Embedding生成服务 (集成模型API)
- [ ] 向量相似度搜索优化

### 7.2 记忆管理增强
- [x] 记忆写入去重（`content_hash` 指纹 + 重复更新 last_seen/access_count）
- [x] 自动提取/召回/敏感过滤/审计开关（`memory_settings`）
- [x] 敏感信息过滤（只作用于持久化/审计/召回，不影响工具调度）
- [x] 记忆审计日志（`memory_audit_logs`）
- [x] 混合检索索引预留（长期记忆异步写入向量库，支持后续 keyword+vector 召回）
- [x] 记忆衰减算法（层面2：按最近访问/出现/创建时间、访问次数、类型权重动态评分）
- [x] 记忆合并策略（层面2：高相似无冲突内容合并，冲突内容保留并审计）
- [x] 记忆优先级排序（层面2：keyword + recency + frequency + type_weight，预留 vector_score）
- [x] 混合检索融合（层面3：keyword + vector + 衰减评分融合，向量服务失败明确降级）
- [x] 召回解释（层面3：返回/记录 keyword、vector、recency、frequency、type 权重组成）
- [x] 记忆容量限制（层面3：超出容量后归档低价值记忆，保留审计）
- [ ] 记忆分类标签

### 7.3 上下文注入
- [ ] 相关记忆检索 (集成到AI助手)
- [ ] 上下文窗口管理
- [ ] 记忆引用追踪
- [ ] 记忆更新触发

### 7.4 前端集成
- [ ] 向量记忆管理页面
- [ ] 记忆搜索界面
- [ ] 记忆可视化 (相似度图谱)

---

## 八、Skill系统增强 🟢 P2

### 8.1 Skill执行引擎
- [x] Skill步骤执行器
- [x] MCP tool执行器
- [x] 可视化测试执行面板
- [x] 执行记录持久化 (skill_executions)
- [x] Skill/MCP 生命周期：`draft/testing/published/disabled`，兼容旧 `active/archived`
- [x] 执行模式：测试默认 `sandbox`，`production` 仅允许 `published`，执行记录写入 `execution_mode`
- [ ] 条件分支支持
- [ ] 循环执行支持
- [ ] 并行步骤执行

### 8.2 Skill模板
- [ ] 模板变量替换
- [ ] 模板继承
- [ ] 模板版本管理
- [ ] 模板市场

### 8.3 Skill调试
- [x] 执行日志记录
- [ ] 断点调试
- [ ] 性能分析
- [ ] 错误追踪

---

## 九、监控与运维 🟢 P2

### 9.1 系统监控
- [ ] 服务健康检查
- [ ] 资源使用监控
- [ ] 性能指标采集
- [ ] 告警规则配置

### 9.2 日志系统
- [ ] 结构化日志
- [ ] 日志聚合
- [ ] 日志查询
- [ ] 日志归档

### 9.3 链路追踪
- [ ] 请求链路追踪
- [ ] 调用关系分析
- [ ] 性能瓶颈定位
- [ ] 错误传播分析

---

## 十、前端增强 ⚪ P3

### 10.1 UI优化
- [ ] 响应式设计优化
- [ ] 暗黑模式
- [ ] 国际化支持
- [ ] 无障碍访问

### 10.2 功能增强
- [x] AI助手浮动组件
- [ ] 实时通知
- [ ] 数据可视化
- [ ] 批量操作
- [ ] 导出功能

### 10.3 用户体验
- [ ] 快捷键支持
- [ ] 搜索功能
- [ ] 收藏功能
- [ ] 历史记录

---

## 十一、部署与扩展 ⚪ P3

### 11.1 容器化
- [ ] Docker镜像构建
- [ ] Docker Compose配置
- [ ] Kubernetes部署
- [ ] Helm Chart

### 11.2 CI/CD
- [ ] 自动化测试
- [ ] 自动化构建
- [ ] 自动化部署
- [ ] 版本管理

### 11.3 高可用
- [ ] 数据库读写分离
- [ ] Redis缓存层
- [ ] 负载均衡
- [ ] 故障转移

---

## 近期开发计划

### 第一阶段 ✅ 已完成
1. ✅ 用户认证系统 (JWT)
2. ✅ RBAC权限实现
3. ✅ SSO单点登录
4. ✅ AI助手对话 (SSE + Tool Calling)

### 第二阶段 🔄 进行中
1. ✅ MCP协议完整实现
2. ✅ 熔断限流机制
3. ✅ 模型配置管理
4. 🟢 **向量记忆系统** ← 当前重点

### 第三阶段 (下一步)
1. 🟢 向量记忆系统完善 (部署向量DB + Embedding)
2. 🟢 记忆注入AI助手上下文
3. 🟢 Skill高级执行 (条件分支/循环)
4. 🟢 监控告警系统

### 第四阶段 (未来)
1. ⚪ 前端全面优化
2. ⚪ 容器化部署
3. ⚪ 高可用架构
