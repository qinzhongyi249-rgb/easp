# EASP Platform - 数据库设计文档

> 最后更新: 2026-06-09

## 数据库信息

```
Host: rm-8vbh4iqcp8534vs5p6o.mysql.zhangbei.rds.aliyuncs.com
Port: 3306
User: easp_dev
Password: Easp_dev123
Database: easp_dev
Charset: utf8mb4
```

---

## 表结构设计

### 1. tenants (租户表)

```sql
CREATE TABLE tenants (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    plan VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    expires_at TIMESTAMP NULL DEFAULT NULL COMMENT '到期时间, NULL=永久',
    max_users INT DEFAULT 0 COMMENT '用户上限, 0=不限',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**字段说明**:
- `id`: 租户唯一标识 (UUID)
- `name`: 租户名称
- `plan`: 套餐计划 (basic/pro/enterprise)
- `status`: 状态 (active/suspended/deleted)
- `expires_at`: 到期时间 (NULL=永久)
- `max_users`: 用户上限 (0=不限)

---

### 2. users (用户表)

```sql
CREATE TABLE users (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    email VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    avatar TEXT,
    phone VARCHAR(50),
    status VARCHAR(50) NOT NULL,
    password_hash VARCHAR(255),
    sso_provider VARCHAR(50),
    sso_user_id VARCHAR(100),
    sso_linked_at TIMESTAMP,
    metadata JSON,
    last_login_at TIMESTAMP,
    login_count INT DEFAULT 0,
    deleted_at TIMESTAMP NULL DEFAULT NULL COMMENT '逻辑删除时间, NULL=未删',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_sso_provider (sso_provider),
    INDEX idx_deleted_at (deleted_at)
);
```

**字段说明**:
- `id`: 用户唯一标识 (UUID)
- `tenant_id`: 所属租户ID
- `email`: 邮箱 (唯一)
- `display_name`: 显示名称
- `password_hash`: 密码哈希
- `sso_provider`: SSO提供商 (wechat/dingtalk/feishu)
- `sso_user_id`: SSO用户ID
- `metadata`: 扩展信息 (JSON)
- `deleted_at`: 逻辑删除时间 (NULL=未删)

---

### 3. roles (角色表)

```sql
CREATE TABLE roles (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN DEFAULT FALSE COMMENT '是否系统级角色',
    tools JSON,
    rate_limit VARCHAR(50),
    data_scope VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_is_system (is_system)
);
```

**字段说明**:
- `id`: 角色唯一标识 (UUID)
- `tenant_id`: 所属租户ID (系统级角色可为NULL)
- `name`: 角色名称
- `is_system`: 是否系统级角色 (true=系统级, false=租户级)
- `tools`: 可访问的工具列表 (JSON数组)
- `rate_limit`: 限流策略
- `data_scope`: 数据权限范围

---

### 4. user_roles (用户角色关联表)

```sql
CREATE TABLE user_roles (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    role_id VARCHAR(36) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user_role (user_id, role_id),
    INDEX idx_user_id (user_id),
    INDEX idx_role_id (role_id)
);
```

---

### 5. connectors (连接器表)

```sql
CREATE TABLE connectors (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    connector_type VARCHAR(50) NOT NULL,
    config JSON,
    status VARCHAR(50) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id)
);
```

---

### 6. mcp_tools (MCP工具表)

```sql
CREATE TABLE mcp_tools (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    connector_id VARCHAR(36),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    tool_type VARCHAR(50) NOT NULL,
    parameters JSON,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_connector_id (connector_id)
);
```

---

### 7. memory_pools (记忆池表)

```sql
CREATE TABLE memory_pools (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    pool_type VARCHAR(50) NOT NULL,
    config JSON,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id)
);
```

---

### 8. memory_entries (记忆条目表)

```sql
CREATE TABLE memory_entries (
    id VARCHAR(36) PRIMARY KEY,
    pool_id VARCHAR(36) NOT NULL,
    type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    metadata JSON,
    sensitivity VARCHAR(20) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_pool_id (pool_id),
    INDEX idx_type (type)
);
```

**字段说明**:
- `type`: 记忆类型 (fact/preference/event/skill)
- `sensitivity`: 敏感度 (low/medium/high)
- `content`: 记忆内容

---

### 9. memory_vectors (向量记忆表) ✅ 新增

```sql
CREATE TABLE memory_vectors (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    pool_id VARCHAR(36),
    content TEXT NOT NULL COMMENT '记忆内容',
    type VARCHAR(50) DEFAULT 'fact' COMMENT '记忆类型 (fact/preference/event/skill)',
    sensitivity VARCHAR(20) DEFAULT 'normal' COMMENT '敏感度 (low/normal/high)',
    embedding BLOB COMMENT '向量嵌入 (由Embedding服务生成)',
    metadata JSON COMMENT '扩展元数据',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_pool_id (pool_id),
    INDEX idx_type (type)
);
```

**字段说明**:
- `content`: 记忆内容文本
- `type`: 记忆类型
- `sensitivity`: 敏感度
- `embedding`: 向量嵌入 (用于相似度搜索)
- `metadata`: 扩展元数据 (JSON)

**使用场景**:
- AI助手上下文注入 (相关记忆检索)
- 语义搜索 (相似问题匹配)
- 知识库问答

---

### 10. skills (Skill模板表)

```sql
CREATE TABLE skills (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    version VARCHAR(20) NOT NULL,
    triggers JSON,
    steps JSON NOT NULL,
    permission_topology JSON,
    status VARCHAR(50) NOT NULL,
    created_by VARCHAR(36),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_status (status)
);
```

**字段说明**:
- `triggers`: 触发条件 (JSON)
- `steps`: 执行步骤 (JSON数组)
- `permission_topology`: 权限拓扑 (JSON)
- `status`: 状态 (draft/active/archived)

---

### 11. skill_executions (Skill执行记录表) ✅ 新增

```sql
CREATE TABLE skill_executions (
    id VARCHAR(36) PRIMARY KEY,
    skill_id VARCHAR(36) NOT NULL,
    tenant_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36),
    inputs JSON COMMENT '执行输入参数',
    outputs JSON COMMENT '执行输出结果',
    status VARCHAR(50) NOT NULL COMMENT '执行状态 (running/completed/failed)',
    error TEXT COMMENT '错误信息',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL DEFAULT NULL,
    duration_ms INT COMMENT '执行耗时(毫秒)',
    INDEX idx_skill_id (skill_id),
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_status (status),
    INDEX idx_started_at (started_at)
);
```

**字段说明**:
- `skill_id`: 关联的Skill ID
- `tenant_id`: 所属租户ID
- `user_id`: 执行用户ID
- `inputs`: 执行输入参数 (JSON)
- `outputs`: 执行输出结果 (JSON)
- `status`: 执行状态 (running/completed/failed)
- `error`: 错误信息 (失败时)
- `duration_ms`: 执行耗时 (毫秒)

---

### 12. audit_logs (审计日志表)

```sql
CREATE TABLE audit_logs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(36),
    agent_id VARCHAR(100),
    tool VARCHAR(200) NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(200),
    detail JSON,
    decision VARCHAR(50),
    result VARCHAR(50),
    duration_ms INT,
    ip VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_user_id (user_id),
    INDEX idx_tool (tool),
    INDEX idx_created_at (created_at)
);
```

---

### 13. model_providers (模型提供商表)

```sql
CREATE TABLE model_providers (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(100),
    base_url TEXT NOT NULL,
    api_key TEXT NOT NULL,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id)
);
```

---

### 14. model_configs (模型配置表)

```sql
CREATE TABLE model_configs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL,
    provider_id VARCHAR(36) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    display_name VARCHAR(100),
    temperature DECIMAL(3,2) DEFAULT 1.00,
    max_tokens INT DEFAULT 4096,
    is_default BOOLEAN DEFAULT FALSE,
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id),
    INDEX idx_provider_id (provider_id)
);
```

**业务规则**:
- 默认模型互斥: 同一租户下只能有一个 is_default=true 的配置
- 供应商禁用检测: 如果 provider.enabled=false, 该供应商下的配置不可用
- 禁止硬编码fallback: 必须明确指定使用哪个模型配置

---

### 15. tenant_sso_configs (租户SSO配置表) ✅ 新增

```sql
CREATE TABLE tenant_sso_configs (
    id VARCHAR(36) PRIMARY KEY,
    tenant_id VARCHAR(36) NOT NULL UNIQUE,
    provider VARCHAR(50) NOT NULL COMMENT 'SSO提供商 (wechat/dingtalk/feishu/custom)',
    client_id VARCHAR(255),
    client_secret VARCHAR(255),
    redirect_uri VARCHAR(500),
    authorize_url VARCHAR(500),
    token_url VARCHAR(500),
    userinfo_url VARCHAR(500),
    scopes VARCHAR(500),
    extra_config JSON COMMENT '额外配置',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_id (tenant_id)
);
```

**字段说明**:
- `tenant_id`: 所属租户ID (唯一)
- `provider`: SSO提供商类型
- `client_id`: OAuth2 Client ID
- `client_secret`: OAuth2 Client Secret (加密存储)
- `redirect_uri`: 回调地址
- `authorize_url`: 授权URL
- `token_url`: Token URL
- `userinfo_url`: 用户信息URL
- `scopes`: 授权范围
- `extra_config`: 额外配置 (JSON)

---

### 16. 其他表

- `circuit_breaker_states`: 熔断器状态表
- `abac_rules`: ABAC规则表
- `sso_providers`: SSO提供商配置表

---

## ER关系图

```
tenants (1) ─────┬───── (N) users
                 │
                 ├───── (N) roles
                 │
                 ├───── (N) connectors
                 │           │
                 │           └─── (N) mcp_tools
                 │
                 ├───── (N) memory_pools
                 │           │
                 │           └─── (N) memory_entries
                 │
                 ├───── (N) memory_vectors (向量记忆)
                 │
                 ├───── (N) skills
                 │           │
                 │           └─── (N) skill_executions
                 │
                 ├───── (N) audit_logs
                 │
                 ├───── (N) model_providers
                 │           │
                 │           └─── (N) model_configs
                 │
                 ├───── (1) tenant_sso_configs
                 │
                 └───── (N) abac_rules

users (N) ──────── (N) roles  [通过 user_roles 关联]
```

---

## 数据隔离策略

### 租户级隔离
- 所有业务表都有 `tenant_id` 字段
- 查询时必须添加 `WHERE tenant_id = ?` 条件
- API中间件自动注入租户ID

### 用户级隔离
- 用户只能访问自己租户的数据
- 通过JWT token中的 `tenant_id` 和 `user_id` 进行权限校验

### 系统级数据
- 系统级角色 (`is_system=true`) 不关联租户
- 系统管理员可以访问所有租户数据

---

## 数据完整性约束

### 外键约束
- `users.tenant_id` → `tenants.id`
- `user_roles.user_id` → `users.id`
- `user_roles.role_id` → `roles.id`
- `connectors.tenant_id` → `tenants.id`
- `mcp_tools.tenant_id` → `tenants.id`
- `mcp_tools.connector_id` → `connectors.id`
- `memory_pools.tenant_id` → `tenants.id`
- `memory_entries.pool_id` → `memory_pools.id`
- `memory_vectors.tenant_id` → `tenants.id`
- `skills.tenant_id` → `tenants.id`
- `skill_executions.skill_id` → `skills.id`
- `audit_logs.tenant_id` → `tenants.id`
- `model_providers.tenant_id` → `tenants.id`
- `model_configs.provider_id` → `model_providers.id`
- `tenant_sso_configs.tenant_id` → `tenants.id`

### 唯一约束
- `users.email` (全局唯一)
- `user_roles.(user_id, role_id)` (用户角色组合唯一)
- `tenant_sso_configs.tenant_id` (每租户一个SSO配置)

### 逻辑删除
- `users.deleted_at` (NULL=未删, 非NULL=已删)
- 查询未删除用户: `WHERE deleted_at IS NULL`

### 租户状态校验
- 注册/登录时校验: `tenants.status = 'active' AND (tenants.expires_at IS NULL OR tenants.expires_at > NOW())`
- 用户上限校验: `SELECT COUNT(*) FROM users WHERE tenant_id = ? AND deleted_at IS NULL < tenants.max_users`
