# 微信端调用 Qoder CLI 修改代码方案

## 1. 背景与目标

### 现状
- 微信端用户通过 Hermes Gateway 与 AI 交互
- 当前模型：NVIDIA Qwen3.5-397b（纯对话）
- Qoder CLI 已安装并认证，但仅能在本地终端使用

### 目标
微信用户发送代码修改指令时，Hermes 自动调用 Qoder CLI 执行实际代码修改，并返回结果。

### 典型场景
```
用户： "帮我把用户登录接口加上邮箱验证"
→ Hermes 识别为代码修改任务
→ 调用 Qoder CLI quest 模式执行
→ 返回修改的 code diff 和文件列表
→ 用户确认后可选提交 Git
```

---

## 2. 架构设计

### 2.1 整体流程

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  微信用户    │ ──▶ │  微信网关    │ ──▶ │  EASP 后端   │ ──▶ │  Qoder CLI   │
│  (发送消息)  │     │  (接收消息)  │     │  (路由判断)  │     │  (执行修改)  │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
       ▲                                                              │
       │                                                              ▼
       │                                                    ┌──────────────┐
       │                                                    │  Git 工作区  │
       │                                                    │  (代码修改)  │
       │                                                    └──────────────┘
       │
       │     ┌──────────────┐     ┌──────────────┐
       └──── │  微信网关    │ ◀── │  EASP 后端   │
             │  (返回结果)  │     │  ( Diff + 状态)│
             └──────────────┘     └──────────────┘
```

### 2.2 组件职责

| 组件 | 职责 |
|------|------|
| 微信网关 | 接收/发送微信消息，透传到 EASP 后端 |
| EASP 后端 | 识别代码任务、调用 Qoder CLI、权限校验、Git 操作 |
| Qoder CLI | 实际执行代码修改（quest/chat/review 模式） |
| Git 工作区 | 存储修改、生成 Diff、支持回滚 |

---

## 3. 详细设计

### 3.1 任务识别规则

Hermes Gateway 需要识别**代码修改任务** vs **普通对话**：

| 特征 | 代码修改任务 | 普通对话 |
|------|-------------|---------|
| 关键词 | "修改"、"添加"、"删除"、"重构"、"修复 bug" | "查询"、"为什么"、"如何" |
| 目标对象 | 文件路径、函数名、接口名 | 通用问题 |
| 示例 | "把 auth.go 的登录接口加上邮箱验证" | "用户登录流程是什么" |

**识别逻辑**（后端）：
```go
func isCodeModificationTask(content string) bool {
    codeKeywords := []string{"修改", "添加", "删除", "重构", "修复", "优化"}
    codeObjects := []string{"接口", "函数", "方法", "文件", "class", "struct"}

    hasKeyword := containsAny(content, codeKeywords)
    hasObject := containsAny(content, codeObjects)

    return hasKeyword && hasObject
}
```

---

### 3.2 API 设计

#### 新接口：`POST /api/v1/qoder/execute`

**请求**：
```json
{
  "tenant_id": "xxx",
  "user_id": "xxx",
  "task": "修改用户登录接口，添加邮箱验证",
  "project": "easp",
  "project_path": "/home/workCode/easp",
  "files": ["./internal/handler/auth.go"],  // 可选，Qoder 自动识别
  "mode": "quest",  // quest | chat | review | refactor
  "options": {
    "model": "Qwen3.7-Max",
    "create_git_branch": true,
    "auto_commit": false
  }
}
```

**响应**：
```json
{
  "status": "success",
  "execution_id": "exec_20260611_123456",
  "mode": "quest",
  "result": {
    "modified_files": [
      {
        "path": "./internal/handler/auth.go",
        "status": "modified",
        "diff_preview": "@@ -45,6 +45,12 @@\n func Login(...) {\n+    // 邮箱验证\n+    if !validateEmail(req.Email) {\n+        return Error(\"invalid email\")\n+    }\n     ...\n }"
      }
    ],
    "git_branch": "qoder/20260611_123456",
    "summary": "已添加邮箱验证逻辑到登录接口",
    "warnings": ["需要添加邮箱验证的单测"]
  }
}
```

**错误响应**：
```json
{
  "status": "error",
  "error_code": "QODER_AUTH_FAILED",
  "message": "Qoder CLI 未登录或认证过期",
  "action": "请运行 qoderclicn auth login 重新认证"
}
```

---

### 3.3 后端实现（伪代码）

```go
// gateway/platforms/weixin.go
func handleWechatMessage(msg *WechatMessage) {
    // 1. 识别任务类型
    if isCodeModificationTask(msg.Content) {
        // 2. 调用 Qoder CLI 接口
        result, err := callQoderCLI(msg.Content, msg.From)
        if err != nil {
            sendWechatReply(msg.From, "❌ 代码修改失败: "+err.Error())
            return
        }

        // 3. 返回结果（带 Diff）
        reply := formatQoderResult(result)
        sendWechatReply(msg.From, reply)
    } else {
        // 普通对话，走原有流程（NVIDIA Qwen3.5）
        callNormalModel(msg)
    }
}

// internal/qoder/executor.go
func ExecuteTask(req *QoderRequest) (*QoderResult, error) {
    // 1. 权限校验
    user, err := checkUserPermission(req.UserID, req.Project)
    if err != nil {
        return nil, fmt.Errorf("权限不足：%w", err)
    }

    // 2. 创建 Git 分支（保护主分支）
    branchName := fmt.Sprintf("qoder/%s_%s", time.Now().Format("20060102"), uuid.New().Short())
    if req.Options.CreateGitBranch {
        err := git.CreateBranch(req.ProjectPath, branchName)
        if err != nil {
            return nil, fmt.Errorf("创建分支失败：%w", err)
        }
    }

    // 3. 调用 Qoder CLI
    cmdArgs := []string{"-p"}  // print mode, non-interactive
    if req.Mode == "quest" {
        cmdArgs = append(cmdArgs, "quest", req.Task)
    } else if req.Mode == "chat" {
        cmdArgs = append(cmdArgs, "-i", req.Task)
    }

    cmd := exec.Command("qoderclicn", cmdArgs...)
    cmd.Dir = req.ProjectPath
    cmd.Env = append(os.Environ(), fmt.Sprintf("USER=%s", user.Email))

    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("Qoder 执行失败：%w", err)
    }

    // 4. 解析输出，提取修改的文件和 Diff
    result := parseQoderOutput(output)
    result.GitBranch = branchName

    // 5. （可选）自动提交
    if req.Options.AutoCommit {
        err := git.CommitChanges(req.ProjectPath, result.ModifiedFiles, req.Task)
        if err != nil {
            return nil, fmt.Errorf("Git 提交失败：%w", err)
        }
    }

    return result, nil
}
```

---

### 3.4 Git 工作流保护

**分支策略**：
```
main (受保护，禁止直接修改)
  └── qoder/YYYYMMDD_xxx (临时分支，Qoder 修改)
        ├── 用户确认 → merge to main
        └── 用户拒绝 → 删除分支
```

**API 扩展**：
```go
// POST /api/v1/qoder/commit
{
  "execution_id": "exec_20260611_123456",
  "action": "commit" | "discard",
  "commit_message": "feat: 添加邮箱验证到登录接口"
}
```

---

## 4. 安全与权限

### 4.1 用户角色

| 角色 | 权限 |
|------|------|
| 超级管理员 | 所有项目 + 强制提交 |
| 开发者 | 授权项目的修改 + Diff 预览 |
| 普通用户 | 无代码修改权限（仅对话） |

### 4.2 项目隔离

- 微信用户绑定到租户（tenant_id）
- 租户关联可访问的项目列表
- Qoder CLI 只能操作授权目录

### 4.3 审计日志

```go
type QoderAuditLog struct {
    UserID       string
    Project      string
    Task         string
    ModifiedFiles []string
    GitBranch    string
    Action       string // executed/committed/discarded
    Timestamp    time.Time
}
```

---

## 5. 前端（微信）交互设计

### 5.1 消息格式

**用户发送**：
> 修改登录接口，添加邮箱验证

**Hermes 回复**：
```
🔧 代码修改任务已执行

📝 任务：修改登录接口，添加邮箱验证
📂 修改文件：
  - internal/handler/auth.go (+12 行)

📊 Diff 预览：
  @@ -45,6 +45,12 @@
   func Login(...) {
  +    // 邮箱验证
  +    if !validateEmail(req.Email) {
  +        return Error("invalid email")
  +    }
       ...
   }

🌿 Git 分支：qoder/20260611_123456

[确认提交] [查看完整 Diff] [放弃修改]
```

### 5.2 按钮交互

- **确认提交** → 调用 `/api/v1/qoder/commit` → merge to main
- **查看完整 Diff** → 发送详细 Diff 消息
- **放弃修改** → 删除临时分支

---

## 6. 实施计划

### Phase 1：后端基础（1-2 天）
- [ ] 添加 `/api/v1/qoder/execute` 接口
- [ ] 实现 Qoder CLI 调用封装
- [ ] Git 分支保护逻辑
- [ ] 权限校验

### Phase 2：微信集成（1 天）
- [ ] 微信网关识别代码任务
- [ ] 消息格式美化
- [ ] 按钮交互（确认/放弃）

### Phase 3：安全与审计（1 天）
- [ ] 审计日志
- [ ] 租户/项目隔离
- [ ] 错误处理与回滚

### Phase 4：测试与优化（1 天）
- [ ] 单元测试
- [ ] 真实场景测试
- [ ] 性能优化

**总计**：4-5 天

---

## 7. 风险与注意事项

### 7.1 风险

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Qoder CLI 修改错误 | 代码损坏 | Git 分支保护 + 用户确认 |
| 并发冲突 | 多人同时修改 | 分支隔离 + 锁机制 |
| 认证过期 | 执行失败 | 定期刷新 token + 错误提示 |
| 上下文超限 | 大任务失败 | 任务拆分 + 文件限制 |

### 7.2 注意事项

1. **Qoder CLI 版本**：当前 v1.0.17，需定期检查更新
2. **项目路径**：硬编码 `/home/workCode/easp`，需支持配置
3. **模型选择**：Quest 模式默认用 Qwen3.7-Max，费用较高
4. **微信消息长度**：Diff 可能超长，需分页或摘要

---

## 8. 决策点

### 8.1 可选配置

| 配置项 | 选项 A | 选项 B | 推荐 |
|--------|-------|-------|------|
| 自动提交 | 用户确认后自动 merge | 始终手动 merge | B（更安全） |
| Git 分支 | 每个任务新建分支 | 共享 qoder/dev 分支 | A（隔离更好） |
| Diff 展示 | 微信内直接显示 | 生成链接查看 | A（体验更好） |
| 文件限制 | 单次最多修改 5 个文件 | 无限制 | A（控制风险） |

### 8.2 待确认

- [ ] 微信端按钮交互是否支持？（需确认微信 API）
- [ ] 是否需要异步任务模式？（大任务可能超时）
- [ ] 是否需要 Code Review 前置审批？

---

## 9. 下一步

阅读此文档后，请确认：

1. **是否实现**？→ 是/否/修改后实现
2. **优先级**？→ 高/中/低
3. **配置选择**？→ 参考 8.1 节
4. **其他需求**？→ 补充到此文档

---

*文档版本：v1.0*
*创建时间：2026-06-11*
*作者：Hermes Agent*