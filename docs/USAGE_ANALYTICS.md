# EASP 用量分析设计方案

> 最后更新: 2026-06-12

## 目标

新增独立菜单「用量分析」，支持租户维度查看模型 Token 消耗、模型调用次数、MCP 工具调用次数、Skill 调用次数，并支持按日/月/年汇总和明细查询。仪表盘只保留摘要，复杂筛选和图表进入独立页面。

## 核心问题

当前已有 `model_usage` 和 `api_usage`：

- `model_usage` 已记录 tenant/user/provider/model/input_tokens/output_tokens/cached_tokens/total_tokens/latency/endpoint/created_at。
- `api_usage` 已记录 API 请求次数和耗时。

不足：

1. `model_usage` 缺少功能来源，如 AI助手、Embed、MCP、Skill。
2. 缺少 MCP 工具、Skill 的统一调用次数明细表。
3. 租户管理页已有简版使用量 Drawer，但不支持年月日粒度、图表、明细、多来源分析。

## 数据库设计

### 扩展 model_usage

新增字段：

```sql
ALTER TABLE model_usage
  ADD COLUMN source VARCHAR(32) NOT NULL DEFAULT 'unknown' COMMENT '调用来源: ai_assistant/embed/mcp/skill/api/unknown',
  ADD COLUMN source_name VARCHAR(100) NOT NULL DEFAULT '' COMMENT '来源名称',
  ADD COLUMN resource_type VARCHAR(32) NOT NULL DEFAULT '' COMMENT '资源类型: assistant/embed/mcp_tool/skill/builtin_tool',
  ADD COLUMN resource_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '资源ID',
  ADD COLUMN request_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求链路ID';
```

字段说明：

| 字段 | 说明 |
|---|---|
| source | 功能来源，如 `ai_assistant`、`embed`、`mcp_api`、`skill`、`manual` |
| source_name | 展示名，如「AI助手」「嵌入式聊天」「角色分配」 |
| resource_type | 资源类型，如 `skill`、`mcp_tool`、`builtin_tool` |
| resource_id | 资源ID，可为空 |
| request_id | 一次请求链路ID，用于串联模型调用和工具调用 |

### 新增 tool_call_usage

```sql
CREATE TABLE IF NOT EXISTS tool_call_usage (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  tenant_id VARCHAR(36) NOT NULL,
  user_id VARCHAR(36) NOT NULL DEFAULT '',
  resource_type VARCHAR(32) NOT NULL COMMENT 'mcp_tool/skill/builtin_tool',
  resource_id VARCHAR(64) NOT NULL DEFAULT '',
  resource_name VARCHAR(128) NOT NULL DEFAULT '',
  source VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'assistant/skill/mcp_api/embed/manual',
  status VARCHAR(32) NOT NULL DEFAULT 'success',
  latency_ms INT NOT NULL DEFAULT 0,
  request_id VARCHAR(64) NOT NULL DEFAULT '',
  error_message TEXT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_tenant_date (tenant_id, created_at),
  INDEX idx_resource (tenant_id, resource_type, resource_id),
  INDEX idx_source (tenant_id, source, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

## 埋点规则

| 场景 | model_usage | tool_call_usage |
|---|---|---|
| AI助手普通对话 | source=ai_assistant | 无 |
| AI助手调用内置工具 | source=ai_assistant | resource_type=builtin_tool, source=assistant |
| AI助手调用 MCP 工具 | source=ai_assistant, resource_type=mcp_tool | resource_type=mcp_tool, source=assistant |
| AI助手调用 Skill | source=ai_assistant, resource_type=skill | resource_type=skill, source=assistant |
| Skill 内部调用 MCP 工具 | 通常无模型 token | resource_type=mcp_tool, source=skill |
| 页面手动执行 Skill | 通常无模型 token | resource_type=skill, source=manual |
| Embed 聊天 | source=embed | 可按后续工具能力记录 |
| MCP 协议直接调用工具 | 通常无模型 token | resource_type=mcp_tool, source=mcp_api |

## 后端 API

### 用量分析总览

```http
GET /api/v1/tenants/:tenantId/usage/analytics
```

查询参数：

| 参数 | 说明 |
|---|---|
| start_date | 开始日期，格式 YYYY-MM-DD |
| end_date | 结束日期，格式 YYYY-MM-DD |
| granularity | `day` / `month` / `year` |
| source | 可选，功能来源 |
| model_name | 可选，模型名 |
| resource_type | 可选，资源类型 |
| page | 明细页码 |
| page_size | 明细条数 |

返回内容：

- summary：总输入/输出/总 tokens、模型调用次数、工具调用次数、Skill 调用次数、平均耗时
- trend：按日期/月/年聚合的 token 趋势
- by_model：按模型聚合
- by_source：按功能来源聚合
- by_tool：MCP 工具和 Skill 调用排行
- details：明细列表

### 仪表盘摘要

```http
GET /api/v1/tenants/:tenantId/usage/summary
```

返回今日/本月摘要，用于 Dashboard 卡片。

## 前端设计

新增：

```txt
frontend/src/api/usageAnalytics.ts
frontend/src/pages/UsageAnalytics.tsx
```

修改：

```txt
frontend/src/App.tsx
frontend/src/layouts/MainLayout.tsx
frontend/src/pages/Dashboard.tsx
```

页面结构：

1. 筛选栏：日期范围、粒度、来源、资源类型、模型。
2. 指标卡片：总 tokens、输入/输出 tokens、模型调用次数、MCP 工具调用、Skill 调用。
3. 图表：先用 Antd + CSS 简易柱状图，避免新增大依赖；后续可切换 ECharts。
4. 排行：按模型、来源、工具/技能排行。
5. 明细表：时间、用户、来源、模型、资源、tokens、耗时。

## 权限与菜单

- 新菜单 key：`usage-analytics`
- 超级管理员可查看所有租户，普通租户用户只查看当前租户。
- 前端菜单按 `hasTool('usage-analytics')` 控制；管理员 `tools=["*"]` 自动可见。

## 验证标准

1. 后端 `go build` 通过。
2. 前端 `npm run build` 通过。
3. `/usage-analytics` 页面可加载。
4. 筛选今日/本月能返回数据。
5. AI助手调用后 `model_usage.source=ai_assistant` 有记录。
6. 手动执行 Skill 后 `tool_call_usage.resource_type=skill` 有记录。
7. Skill 内部调用 MCP/内置工具后 `tool_call_usage` 有对应记录。
8. Dashboard 显示今日/本月 Token 和工具/Skill 调用摘要。
