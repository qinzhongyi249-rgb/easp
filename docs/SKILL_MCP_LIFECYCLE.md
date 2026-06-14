# Skill/MCP 生命周期与执行模式

> 最后更新: 2026-06-13

## 目标

避免 AI 助手或测试入口默认触发生产副作用。Skill 和 MCP Tool 均按生命周期发布，执行入口显式区分测试/正式模式。

## 生命周期状态

| 状态 | 含义 | 允许执行模式 |
|------|------|--------------|
| `draft` | 草稿，默认新建状态 | `sandbox` / `dry_run` |
| `testing` | 测试中 | `sandbox` / `dry_run` |
| `published` | 已发布，可正式调用 | `sandbox` / `dry_run` / `production` |
| `disabled` | 已停用 | 禁止执行 |

兼容旧数据：
- `active` 归一为 `published`
- `archived` 归一为 `disabled`

## 执行模式

| 模式 | 含义 | 副作用 |
|------|------|--------|
| `sandbox` | 沙箱测试，默认模式 | 跳过 MCP/HTTP 等外部副作用 |
| `dry_run` | 预演校验 | 跳过 MCP/HTTP 等外部副作用 |
| `production` | 正式执行 | 只允许 `published`，会触发真实调用 |

空执行模式统一归一为 `sandbox`。

## 后端收口规则

- AI 创建 Skill/MCP 默认 `draft`。
- AI 可见的动态 `skill_*` / `mcp_*` 工具只暴露 `published`（兼容旧 `active`）。
- Skill 手动执行、AI `execute_skill`、MCP 直接调用、Skill 内部 MCP 调用都必须校验生命周期。
- `production` 只允许 `published`；`disabled/archived` 禁止任何执行。
- `sandbox/dry_run` 当前返回预演结果或跳过外部调用，后续再接连接器沙箱路由。
- `skill_executions.execution_mode` 记录本次执行模式，前端执行结果与历史列表展示该字段。

## 前端行为

- Skill 新建默认 `draft`。
- Skill/MCP Tool 编辑页显示 `draft/testing/published/disabled`。
- Skill 测试弹窗默认 `sandbox`。
- 未发布 Skill 的 `production` 选项禁用，并提示“正式执行需要先发布”。
- 执行结果与执行历史显示执行模式。

## 后续扩展

- 连接器按环境拆分 sandbox/production。
- 高风险工具执行审批流。
- 发布前校验 input_schema、步骤可达性与权限依赖。
