# EASP AI助手接入手册

> 本文面向第三方业务系统开发者，说明如何把 EASP AI 助手嵌入到 H5/PC 页面。
> 管理后台「AI 助手」页弹窗会按当前租户动态生成 JS SDK、iframe、token exchange 代码片段。

## 1. 接入流程

1. EASP 管理后台创建「嵌入接入应用」，保存一次性展示的 `App Secret`。
2. 业务系统后端保存 `app_id` / `app_secret`。
3. 先通过「用户接入手册」的 `/api/v1/embed/users/sync` 异步/初始化同步外部用户。
4. 业务系统后端为当前登录用户向 EASP 换取 `easp-api-token`；如果助手需要代当前用户调用业务系统 API，同时传入当前业务用户的 `external_access_token`。
5. 业务系统前端通过 JS SDK 或 iframe 嵌入助手，只接收业务后端返回的 `easp-api-token`。
6. 助手接口使用 `easp-api-token`，真实权限仍由 EASP 用户/角色/工具/Skill/MCP 体系实时判断。

## 2. 前置条件：外部用户已同步

`/api/v1/embed/token/exchange` 不默认自动创建用户。当前 `external_user_id` 必须已经通过 `/api/v1/embed/users/sync` 同步，或已经存在有效的外部用户绑定关系。

## 3. 服务端换 token

```http
POST /api/v1/embed/token/exchange
X-EASP-App-Id: <app_id>
X-EASP-Timestamp: <unix_seconds>
X-EASP-Nonce: <random_nonce>
X-EASP-Signature: <hmac_sha256_signature>
Content-Type: application/json
```

请求体：

```json
{
  "tenant_id": "tenant_xxx",
  "external_system": "crm",
  "external_user_id": "u_10001",
  "external_access_token": "<current_business_user_token>",
  "external_token_expires_at": 1790000000,
  "auto_create_user": true,
  "default_role_ids": ["role-xxx"]
}
```

签名规则：

```text
secret_hash = SHA256(app_secret)
payload = app_id=...&timestamp=...&nonce=...&external_system=...&external_user_id=...&tenant_id=...
signature = HMAC-SHA256(secret_hash, payload)
```

> `App Secret` 只能放在业务系统后端，不能出现在 H5、PC 前端、JS SDK 参数或 iframe URL 中。
>
> `external_access_token` 是当前业务系统登录用户的业务 token，仅在业务系统后端换取 `easp-api-token` 时传给 EASP。EASP 不会把它返回给浏览器，也不会把明文写入 `easp-api-token`；`easp-api-token` 只携带不可反推的引用。需要代用户调用业务系统 API 的连接器，应在 EASP 连接器配置中使用 `credential_mode=user_token`，并由 `user_token_header` / `user_token_prefix` 控制注入方式。
>
> **auto_create_user**：开启后，如果未找到匹配的外部用户绑定，EASP 会自动创建 EASP 用户并绑定，不需要提前手动导入；`default_role_ids` 必须同时配置，分配默认租户角色。

## 4. JS SDK 嵌入

```html
<script src="https://easp.example.com/embed/assistant.js"></script>
<script>
  EASPAssistant.init({
    baseUrl: 'https://easp.example.com',
    tenantId: 'tenant_xxx',
    tokenProvider: async () => {
      const res = await fetch('/api/easp-assistant-token', { method: 'POST' });
      const data = await res.json();
      return data.easp_api_token;
    },
    executionMode: 'normal', // 可选: 'normal' 真实调用工具 / 'sandbox' 只规划不执行
    user: {
      external_system: 'crm',
      external_user_id: 'u_10001',
      display_name: '张三'
    }
  });
</script>
```

**JS SDK 配置参数：**

| 参数 | 说明 | 是否必填 |
|------|------|----------|
| `baseUrl` | EASP 平台访问地址（不含结尾 `/`） | ✅ 必填 |
| `tenantId` | 租户 ID | ✅ 必填 |
| `tokenProvider` | 获取 `easp-api-token` 的异步函数 | ✅ 必填 |
| `executionMode` | `normal`（真实执行工具）/ `sandbox`（只规划不执行） | 否，默认 `normal` |
| `user` | 当前外部用户信息 `{ external_system, external_user_id, display_name }` | ✅ 必填 |

## 5. iframe 嵌入

```html
<iframe
  id="easp-assistant"
  src="https://easp.example.com/embed/assistant-frame.html?tenant_id=tenant_xxx"
  style="position:fixed;right:20px;bottom:20px;width:420px;height:680px;border:0;z-index:99999;"
></iframe>
```

## 6. 嵌入式助手接口

```http
POST /embed/v1/assistant/chat
GET  /embed/v1/assistant/conversations
GET  /embed/v1/assistant/conversations/{conversation_id}/messages
```

**聊天请求体字段：**

| 字段 | 说明 | 是否必填 |
|------|------|----------|
| `session_id` | 会话 ID，不传自动新建，用于历史上下文 | 否 |
| `message` | 用户当前消息 | ✅ 必填 |
| `assistant_name` | 自定义助手名称 | 否 |
| `execution_mode` | `normal` 真实调用工具 / `sandbox` 只规划不执行 | 否，默认 `normal` |
| `page_context` | 当前页面上下文对象，帮助 AI 理解问题 | 否 |

这些接口只接受 `easp-api-token`，不接受后台管理 JWT。

## 7. 自测清单

- `/embed/assistant.js` 可访问。
- `/embed/assistant-frame.html` 可访问。
- 外部用户已通过 `/api/v1/embed/users/sync` 同步。
- token exchange 返回 `easp-api-token`。
- 浏览器 Network 中助手聊天接口返回 200。
- 审计日志能看到外部系统、外部用户、app_id。
