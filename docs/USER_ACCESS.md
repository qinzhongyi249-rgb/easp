# EASP 用户接入手册

> 本文面向第三方业务系统开发者，说明如何把业务系统里的外部用户、组织属性、微信/飞书等第三方身份同步到 EASP。
> 用户同步是业务系统后端的初始化/异步动作，**不使用 EASP 后台 JWT**；必须使用「嵌入接入应用」的 `app_id` + `app_secret` 做服务端签名。

## 1. 接入目标

业务系统需要完成：

1. 在 EASP 管理后台创建「嵌入接入应用」，保存一次性展示的 `App Secret`。
2. 业务系统后端保存 `app_id` / `app_secret`。
3. 业务系统后端通过 `/api/v1/embed/users/sync` 异步/初始化同步用户。
4. 为用户提供稳定的 `external_user_id`，建议同时提供稳定的 `user_uid`。
5. 如果用户需要账号密码登录 EASP 控制台，必须提供稳定的 `account`，并配置 `password` 或 `default_password`。
6. `email`、`phone` 只是用户属性信息，不作为登录账号，也不做唯一性校验，可后续修改/变更/重复。
7. 如有微信、飞书等身份，把身份信息写入 `identities`。
8. EASP 后端负责租户校验、签名校验、用户上限校验、账号唯一性校验与幂等绑定。

## 2. 必传项、登录账号和密码规则

### 2.1 最小必传项

请求体顶层必须传：

| 字段 | 必填 | 说明 |
|---|---:|---|
| `tenant_id` | 是 | EASP 当前租户 ID |
| `external_system` | 是 | 外部系统标识，必须与嵌入接入应用的 `external_system` 一致 |
| `users` | 是 | 用户数组，数量 1-500 |

每个用户最少必须传：

| 字段 | 必填 | 说明 |
|---|---:|---|
| `external_user_id` | 是 | 外部系统内稳定用户 ID，用于 token exchange 映射 EASP 内部用户 |
| `account` | 建议/登录必需 | EASP 登录账号，租户内唯一。不传时默认使用 `external_user_id` 作为账号 |

### 2.2 用户用哪个账号登录 EASP？

EASP 登录页只使用：

```text
account + password
```

示例：

```json
{
  "account": "zhangsan",
  "password": "Init@123456"
}
```

规则：

- `account` 是租户内唯一登录账号，建议使用员工号、业务系统用户名或其他稳定 ID。
- `email`、`phone` 只是资料属性，不参与账号唯一性校验，不再作为登录账号。
- 如果导入时不传 `account`，EASP 会默认使用 `external_user_id` 作为账号。
- 如果没有配置 `password` / `default_password`，用户可用于嵌入式助手身份映射和审计，但不能用账号密码登录，后续可在用户管理页「重置密码」。

### 2.3 密码配置方式

EASP 支持两种密码配置：

| 字段 | 作用 | 优先级 |
|---|---|---:|
| `users[].password` | 单个用户密码 | 高 |
| `default_password` | 本批次默认密码，适用于未单独配置 `password` 的用户 | 低 |

规则：

- 密码最少 6 位。
- `users[].password` 优先于 `default_password`。
- 如果两者都不传，EASP 会生成不可见随机密码；该用户不能通过账号密码登录，后续可在用户管理页「重置密码」。
- 对已存在/已绑定用户再次同步时，如果传入 `password` 或 `default_password`，会更新该用户密码。
- 批量设置默认密码属于敏感操作，生产环境建议要求用户首次登录后修改密码。

## 3. 用户同步接口

```http
POST /api/v1/embed/users/sync
X-EASP-App-Id: <app_id>
X-EASP-Timestamp: <unix_seconds>
X-EASP-Nonce: <random_nonce>
X-EASP-Signature: <hmac_sha256_signature>
Content-Type: application/json
```

请求体示例：

```json
{
  "tenant_id": "tenant_xxx",
  "external_system": "crm",
  "batch_id": "init-20260622-001",
  "mode": "init",
  "default_password": "Init@123456",
  "users": [
    {
      "external_user_id": "u_10001",
      "account": "zhangsan",
      "user_uid": "crm:u_10001",
      "display_name": "张三",
      "email": "zhangsan@example.com",
      "phone": "13800000000",
      "department": "销售部",
      "position": "客户经理",
      "profile": { "level": "L2" },
      "attributes": { "region": "华北" },
      "identities": [
        { "provider": "wechat", "provider_user_id": "wx_openid_xxx", "display_name": "张三" },
        { "provider": "feishu", "provider_user_id": "ou_xxx", "display_name": "张三" }
      ]
    },
    {
      "external_user_id": "u_10002",
      "account": "lisi",
      "user_uid": "crm:u_10002",
      "display_name": "李四",
      "email": "lisi@example.com",
      "password": "User@123456"
    }
  ]
}
```

响应示例：

```json
{
  "batch_id": "init-20260622-001",
  "summary": { "created": 2, "updated": 0, "bound_existing": 0, "conflict": 0 },
  "items": [
    {
      "external_user_id": "u_10001",
      "user_id": "...",
      "user_uid": "crm:u_10001",
      "account": "zhangsan",
      "login_identifier": "zhangsan",
      "password_configured": true,
      "password_updated": false,
      "status": "created"
    }
  ]
}
```

## 4. 签名规则

EASP 后端会先计算请求体 SHA256，再按固定字段签名：

```text
secret_hash = SHA256(app_secret)
body_sha256 = SHA256(raw_request_body)
payload = app_id=...&timestamp=...&nonce=...&body_sha256=...&external_system=...&tenant_id=...
signature = HMAC-SHA256(secret_hash, payload)
```

说明：

- `timestamp` 默认允许 5 分钟时钟偏差。
- `nonce` 由业务系统后端生成随机值。
- `app_secret` 只能放在业务系统后端，不能出现在 H5/PC 前端、JS SDK 参数、iframe URL 中。
- `tenant_id` 与 `external_system` 必须和该嵌入应用一致。

## 5. Node.js 服务端示例

```js
import crypto from 'node:crypto';

function sha256Hex(text) {
  return crypto.createHash('sha256').update(text).digest('hex');
}

function sign({ appId, appSecret, timestamp, nonce, tenantId, externalSystem, body }) {
  const secretHash = sha256Hex(appSecret);
  const bodyHash = sha256Hex(body);
  const payload = [
    ['app_id', appId],
    ['timestamp', timestamp],
    ['nonce', nonce],
    ['body_sha256', bodyHash],
    ['external_system', externalSystem],
    ['tenant_id', tenantId],
  ].map(([k, v]) => `${k}=${v}`).join('&');
  return crypto.createHmac('sha256', secretHash).update(payload).digest('hex');
}

const body = JSON.stringify({
  tenant_id: process.env.EASP_TENANT_ID,
  external_system: 'crm',
  default_password: 'Init@123456',
  users: [{ external_user_id: 'u_10001', account: 'zhangsan', display_name: '张三' }]
});
```

## 6. 与嵌入式助手 Token Exchange 的关系

用户同步完成后，业务系统后端在当前业务用户打开助手时调用 token exchange：

```json
{
  "tenant_id": "tenant_xxx",
  "external_system": "crm",
  "external_user_id": "u_10001",
  "scope": "assistant:chat"
}
```

EASP 根据 `tenant_id + external_system + external_user_id` 找到内部 `user_id`，再按该内部用户的角色、工具、Skill 权限执行。

> 权限主体始终是 EASP 内部用户，`account` 只用于账号密码登录；业务系统 token 只用于下游业务 API 透传。
