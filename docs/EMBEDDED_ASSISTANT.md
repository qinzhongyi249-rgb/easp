# EASP 嵌入式 AI 助手第三方接入手册

> 本文面向**第三方业务系统开发者/集成方**，说明你需要在自己的业务系统里完成哪些工作，才能把 EASP AI 助手嵌入到 H5/PC 页面。
> EASP 内部的权限、用户绑定、Token 校验、AI 助手、工具/Skill/MCP 执行链路由 EASP 平台负责，第三方系统不要自行实现或绕过。

在线手册：`/docs/embedded-assistant.html`；Markdown 原文：`/docs/embedded-assistant.md`。

---

## 1. 接入前准备

请先从 EASP 租户管理员处获取以下信息：

| 参数 | 说明 | 示例 |
|---|---|---|
| `tenant_id` | EASP 租户 ID | `tenant_xxx` |
| `app_id` | 嵌入接入应用 ID | `app_xxx` |
| `app_secret` | 应用密钥，只展示一次，请放在业务系统服务端 | `easp_secret_xxx` |
| `external_system` | 你的业务系统标识，需要与 EASP 接入应用配置一致 | `crm` |
| EASP 地址 | EASP 平台访问地址 | `https://easp.example.com` |

接入前还需要确认：

1. EASP 管理后台已创建「嵌入接入应用」。
2. 你的业务系统域名已加入应用的 `allowed_origins` 白名单。
3. 需要使用助手的外部用户已在 EASP 完成导入/绑定，并已分配 EASP 角色和工具权限。

> 第三方系统只传递“当前登录的是哪个外部用户”；该用户能用哪些 AI 能力，由 EASP 的用户、角色、工具、Skill、MCP 权限实时决定。

---

## 2. 接入总流程

```text
业务系统用户登录
  ↓
业务系统前端加载 EASP JS SDK 或 iframe
  ↓
业务系统前端请求自己的后端：/api/easp/embed-token
  ↓
业务系统后端用 app_id/app_secret 对当前 external_user_id 签名
  ↓
业务系统后端调用 EASP：POST /api/v1/embed/token/exchange
  ↓
EASP 返回 easp-api-token
  ↓
业务系统前端把 easp-api-token 交给 SDK/iframe
  ↓
用户开始使用嵌入式 AI 助手
```

第三方系统需要实现的只有三件事：

1. **服务端换 Token**：保护 `app_secret`，向 EASP 换取 `easp-api-token`。
2. **页面嵌入助手**：选择 JS SDK 或 iframe。
3. **传递当前外部用户身份**：把你系统里的用户 ID 映射成 `external_user_id`。

---

## 3. 服务端：换取 easp-api-token

### 3.1 你需要提供一个自己的后端接口

建议在业务系统后端提供接口，例如：

```http
POST /api/easp/embed-token
Cookie: <业务系统自己的登录态>
```

这个接口由你的前端调用。接口内部应完成：

1. 校验当前用户已登录你的业务系统。
2. 取出当前用户在你系统中的稳定 ID，作为 `external_user_id`。
3. 用 `app_secret` 生成签名。
4. 调用 EASP token exchange 接口。
5. 把 EASP 返回的 `easp-api-token` 返回给前端。

> 不要把 `app_secret` 放到浏览器、H5、小程序或 App 客户端里。

### 3.2 调用 EASP Token Exchange

```http
POST https://easp.example.com/api/v1/embed/token/exchange
Content-Type: application/json
X-EASP-App-Id: <app_id>
X-EASP-Timestamp: <unix_seconds>
X-EASP-Nonce: <random_string>
X-EASP-Signature: <signature>
```

请求体：

| 字段 | 必填 | 说明 |
|------|------|------|
| `tenant_id` | 是 | EASP 租户 ID |
| `external_system` | 是 | 业务系统标识，和配置一致 |
| `external_user_id` | 是 | 业务用户 ID（需要先导入绑定） |
| `display_name` | 否 | 业务用户显示名 |
| `external_access_token` | 见说明 | **如果连接器配置 `credential_mode = user_token`，必须传此参数**，透传业务用户当前登录 Token，供 MCP 工具调用第三方接口鉴权使用 |
| `external_token_expires_at` | 否 | Token 过期时间戳（Unix 秒），不传默认 2 小时过期 |

成功响应：

```http
HTTP/1.1 200 OK
easp-api-token: <token>
Access-Control-Expose-Headers: easp-api-token
```

```json
{
  "token": "<token>",
  "expires_at": "2026-06-15T16:00:00+08:00",
  "user": {
    "id": "easp-user-id",
    "tenant_id": "tenant_xxx",
    "display_name": "张三"
  }
}
```

### 3.3 签名规则

签名 payload 按固定顺序拼接：

```text
app_id=<app_id>&timestamp=<timestamp>&nonce=<nonce>&external_system=<external_system>&external_user_id=<external_user_id>&tenant_id=<tenant_id>
```

签名密钥：

```text
secret_hash = SHA256(app_secret) 的十六进制字符串
```

签名算法：

```text
signature = HMAC-SHA256(secret_hash, payload) 的十六进制字符串
```

### 3.4 Node.js 示例

```js
import crypto from 'crypto';
import express from 'express';

const router = express.Router();

const EASP_BASE_URL = 'https://easp.example.com';
const TENANT_ID = process.env.EASP_TENANT_ID;
const APP_ID = process.env.EASP_APP_ID;
const APP_SECRET = process.env.EASP_APP_SECRET;
const EXTERNAL_SYSTEM = 'crm';

function signEmbedToken({ externalUserId, timestamp, nonce }) {
  const secretHash = crypto.createHash('sha256').update(APP_SECRET).digest('hex');
  const payload = [
    `app_id=${APP_ID}`,
    `timestamp=${timestamp}`,
    `nonce=${nonce}`,
    `external_system=${EXTERNAL_SYSTEM}`,
    `external_user_id=${externalUserId}`,
    `tenant_id=${TENANT_ID}`
  ].join('&');

  return crypto.createHmac('sha256', secretHash).update(payload).digest('hex');
}

router.post('/api/easp/embed-token', async (req, res) => {
  // 1. 校验你自己系统的登录态
  const currentUser = req.user;
  if (!currentUser) {
    return res.status(401).json({ message: '未登录' });
  }

  // 2. 使用你系统中的稳定用户 ID，对应 EASP 的 external_user_id
  const externalUserId = String(currentUser.id);
  const timestamp = Math.floor(Date.now() / 1000).toString();
  const nonce = crypto.randomBytes(16).toString('hex');
  const signature = signEmbedToken({ externalUserId, timestamp, nonce });

  // 3. 调用 EASP 换取 easp-api-token
  const easpResp = await fetch(`${EASP_BASE_URL}/api/v1/embed/token/exchange`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-EASP-App-Id': APP_ID,
      'X-EASP-Timestamp': timestamp,
      'X-EASP-Nonce': nonce,
      'X-EASP-Signature': signature
    },
    body: JSON.stringify({
      tenant_id: TENANT_ID,
      external_system: EXTERNAL_SYSTEM,
      external_user_id: externalUserId
    })
  });

  const data = await easpResp.json().catch(() => ({}));
  if (!easpResp.ok) {
    return res.status(easpResp.status).json(data);
  }

  // 4. 返回给前端。也可以只返回 token 字段。
  return res.json({
    token: data.token || easpResp.headers.get('easp-api-token'),
    expires_at: data.expires_at,
    user: data.user
  });
});

export default router;
```

---

## 4. 前端：JS SDK 嵌入

适合 H5、移动端、需要读取页面上下文的业务系统。

```html
<script src="https://easp.example.com/embed/assistant.js"></script>
<script>
EASPAssistant.init({
  baseUrl: 'https://easp.example.com',
  tenantId: 'tenant_xxx',
  title: 'AI 助手',
  position: 'right-bottom',
  executionMode: 'normal', // 👈 执行模式：sandbox=纯聊天不调用工具（默认），normal=真实调用用户权限内 MCP/Skill 工具

  // 由你的后端换 token，前端不要接触 app_secret
  tokenProvider: async () => {
    const res = await fetch('/api/easp/embed-token', {
      method: 'POST',
      credentials: 'include'
    });
    if (!res.ok) throw new Error(await res.text());
    const data = await res.json();
    return data.token;
  },

  // 可选：把当前业务页面上下文传给 AI 助手
  pageContextProvider: () => ({
    url: location.href,
    title: document.title,
    page_type: 'customer_detail',
    business_id: window.__BUSINESS_ID__
  })
});
</script>
```

---

## 5. 前端：iframe 嵌入

适合老系统、第三方门户、需要样式强隔离的页面。

```html
<iframe
  id="easp-assistant"
  src="https://easp.example.com/embed/assistant-frame.html?tenant_id=tenant_xxx"
  style="position:fixed;right:20px;bottom:20px;width:420px;height:680px;border:0;z-index:99999;"
></iframe>

<script>
async function initEASPAssistant() {
  const res = await fetch('/api/easp/embed-token', {
    method: 'POST',
    credentials: 'include'
  });
  if (!res.ok) throw new Error(await res.text());

  const data = await res.json();
  const frame = document.getElementById('easp-assistant');

  frame.contentWindow.postMessage({
    type: 'EASP_ASSISTANT_TOKEN',
    token: data.token,
    context: {
      url: location.href,
      title: document.title,
      page_type: 'customer_detail'
    }
  }, 'https://easp.example.com');
}

initEASPAssistant();
</script>
```

更新页面上下文：

```js
document.getElementById('easp-assistant').contentWindow.postMessage({
  type: 'EASP_ASSISTANT_CONTEXT_UPDATE',
  context: {
    url: location.href,
    title: document.title,
    page_type: 'order_detail',
    order_id: 'O-10086'
  }
}, 'https://easp.example.com');
```

退出或切换账号时清空 Token：

```js
document.getElementById('easp-assistant').contentWindow.postMessage({
  type: 'EASP_ASSISTANT_LOGOUT'
}, 'https://easp.example.com');
```

---

## 新增功能说明

### 执行模式（execution_mode）

嵌入式 AI 助手支持两种执行模式：

| 模式 | 说明 | 使用场景 |
|------|------|----------|
| `sandbox` | **默认**，纯聊天不调用任何工具 | 仅问答咨询，不需要调用业务系统接口 |
| `normal` | 真实调用用户权限范围内的 MCP/Skill 工具 | 需要 AI 助手帮用户查询/操作业务系统数据 |

开启方式：在 JS SDK 初始化时加上：
```javascript
executionMode: 'normal'
```

### MCP 工具透传业务 Token

如果你的 MCP 连接器需要使用当前登录用户的 Token 鉴权，请：

1. **EASP 控制台配置连接器**：
   - 凭据模式选择 `user_token`
   - 用户 Token 请求头名称：填写你的业务系统 Token 头（默认 `Authorization`）
   - 用户 Token 前缀：填写前缀（默认 `Bearer`，不需要前缀留空即可）

2. **业务后端在换 Token 时传入**：
   在 `POST /api/v1/embed/token/exchange` 请求体中增加 `external_access_token` 字段，值为当前业务用户的登录 Token。

EASP 会自动暂存 Token 并在 MCP 调用时注入到请求头中，第三方接口就能拿到用户 Token 完成鉴权。

### 新建对话

弹窗右上角自带「新建对话」按钮，用户点击即可清空历史开始新对话，不需要刷新页面。

---

## 7. 历史会话接口（可选）

如果你的业务系统需要展示或归档用户自己的 AI 会话记录，可使用 `easp-api-token` 调用：

```http
GET https://easp.example.com/embed/v1/assistant/conversations
easp-api-token: <token>
```

```http
GET https://easp.example.com/embed/v1/assistant/conversations/:conversationId/messages
easp-api-token: <token>
```

说明：

- 普通嵌入用户只能查询自己的历史会话。
- 租户管理员审计/全量查询请在 EASP 管理后台完成，不要混用嵌入式 Token。

---

## 7. 常见错误

| 错误码 | 含义 | 处理方式 |
|---|---|---|
| `EASP_SIGNATURE_REQUIRED` | 缺少签名请求头 | 检查 `X-EASP-*` 头是否完整 |
| `INVALID_SIGNATURE` | 签名错误 | 检查 payload 顺序、`app_secret`、`external_user_id` 是否一致 |
| `INVALID_TIMESTAMP` | 时间戳过期或服务器时间偏差过大 | 使用当前 Unix 秒级时间戳，确保服务器校时 |
| `INVALID_EMBED_APP` | app 不存在、禁用或与租户/系统不匹配 | 联系 EASP 管理员检查接入应用配置 |
| `ORIGIN_NOT_ALLOWED` | 当前域名不在白名单 | 联系 EASP 管理员添加 `allowed_origins` |
| `TENANT_UNAVAILABLE` | 租户不可用、停用或到期 | 联系 EASP 管理员 |
| `EXTERNAL_USER_NOT_IMPORTED` | 外部用户未导入/未绑定 | 先在 EASP 导入并绑定该用户 |
| `EASP_USER_INACTIVE` | 绑定的 EASP 用户不存在或被停用 | 联系 EASP 管理员启用/修复绑定 |
| `查询失败，当前请求缺少可用的SSO登录用户Token` | MCP 工具需要透传业务 Token，但未传入 `external_access_token` | 在 token exchange 时传入业务用户当前登录 Token |

---

## 8. 第三方系统不要做的事

- 不要在前端保存或传输 `app_secret`。
- 不要用 EASP 后台账号密码在第三方前端模拟登录。
- 不要把 `easp-api-token` 当作 EASP 管理后台 JWT 使用。
- 不要在第三方系统自行决定 EASP 工具、Skill、MCP 权限。
- 不要在外部用户未导入/未绑定时静默创建临时用户。
- 不要把 Token 放到 iframe URL query 中。

---

## 9. 接入自测清单

- [ ] 业务系统后端已配置 `tenant_id/app_id/app_secret/external_system`。
- [ ] `app_secret` 只存在服务端环境变量或密钥管理系统中。
- [ ] 当前业务域名已加入 EASP 应用白名单。
- [ ] 当前测试用户的 `external_user_id` 已在 EASP 导入并绑定。
- [ ] `/api/easp/embed-token` 能成功返回 token。
- [ ] JS SDK 或 iframe 能打开助手并发送消息。
- [ ] Token 过期或 401 时能重新换取 token。
- [ ] 退出/切换账号时会清理旧 token 和会话上下文。
