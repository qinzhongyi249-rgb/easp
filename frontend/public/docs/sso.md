# EASP 租户单点登录（SSO）配置说明

> 本文面向 EASP 租户管理员、第三方系统集成方，说明如何在 EASP 中配置企业单点登录（SSO）。

---

## 1. 功能说明

EASP 支持**租户级单点登录（OAuth2.0）**：

- 企业用户通过企业 IdP（身份提供商）统一登录 EASP。
- 支持常见提供商：企业微信 / 钉钉 / 飞书 / 自定义 OAuth2.0 IdP。
- 每个租户仅支持一个 SSO 配置。
- SSO 登录失败自动回退到标准用户名+密码登录。

---

## 2. 前置准备

你需要从你的 SSO 服务商处获取以下信息：

| 信息项 | 说明 | 示例 |
|---|---|---|
| `client_id` | OAuth 2.0 客户端 ID | `wwxxxxxx` |
| `client_secret` | OAuth 2.0 客户端密钥 | `xxxxxx` |
| `redirect_uri` | 回调地址，必须填写： | `https://easp.example.com/sso/:tenantId/callback` |
| `authorize_url` | 授权 URL | `https://open.weixin.qq.com/connect/oauth2/authorize` |
| `token_url` | 获取 Token URL | `https://api.weixin.qq.com/sns/oauth2/access_token` |
| `userinfo_url` | 获取用户信息 URL | `https://api.weixin.qq.com/sns/userinfo` |
| `scopes` | 授权范围（多个用逗号分隔） | `snsapi_userinfo` |

替换其中的 `:tenantId` 为你的实际租户 ID。

> 如果是企业内网 IdP，请确保 EASP 能访问上述 URL。

---

## 3. 在 EASP 中配置

1. 登录 EASP 管理后台 → 菜单 `SSO 配置`。
2. 选择 SSO 提供商：
   - 企业微信
   - 钉钉
   - 飞书
   - 自定义 OAuth 2.0
3. 填写上方获取到的参数。
4. **开启启用**。
5. 点击「测试连接」，确认配置正确。
6. 点击「保存」。

---

## 4. 访问登录链接

配置完成后，你的用户可以通过以下链接访问 SSO 登录：

```
https://easp.example.com/sso/:tenantId
```

- `tenantId` 替换为你的租户 ID。
- 如果 SSO 配置正确，页面会自动跳转到你的企业 IdP 授权页。
- 用户授权完成后，回调到 EASP，完成登录。
- 如果 SSO 配置异常，会自动回退到标准用户名+密码登录页。

---

## 5. 登录流程

```text
用户访问 /sso/:tenantId
  ↓
EASP 检查租户 SSO 配置
  ↓
如果配置存在且启用 → 跳转到 IdP authorize_url 带 state
如果配置不存在/未启用 → 展示标准登录页
  ↓
用户在 IdP 完成授权 → IdP 重定向到 EASP 回调 /sso/:tenantId/callback
  ↓
EASP 用 code 调用 token_url 获取 access_token
  ↓
EASP 用 access_token 调用 userinfo_url 获取用户信息
  ↓
EASP 根据 userinfo 中的唯一标识匹配/创建用户
  ↓
EASP 签发登录态 → 跳转到 EASP 首页
```

---

## 6. 用户字段映射

EASP 需要从你的用户信息接口中提取以下字段：

| EASP 字段 | 说明 |
|---|---|
| `openid` / `unionid` | 唯一标识，用于关联 EASP 用户 |
| `nickname` | 用户昵称/显示名称 |
| `headimgurl` | 头像 URL（可选） |
| `email` | 邮箱（可选，如果存在） |

EASP 对常见提供商已内置了正确的字段映射，你不需要额外配置。如果使用**自定义 OAuth 2.0**，可以在 `extra_config` 中指定映射关系：

```json
{
  "field_mapping": {
    "user_id": "sub",
    "display_name": "name",
    "email": "email",
    "avatar": "picture"
  }
}
```

---

## 7. 常见问题

### Q1. 测试连接失败怎么办？

A. 请检查：

1. EASP 服务器能否访问你的 `authorize_url` / `token_url` / `userinfo_url`（防火墙/内网配置）。
2. `client_id` / `client_secret` 是否正确复制。
3. `redirect_uri` 是否和你在 IdP 后台填写的完全一致（包括协议 http/https、端口、路径）。
4. 查看浏览器控制台和 EASP 后端日志确认具体错误信息。

### Q2. 回调后提示 "code 无效或已过期"？

A. 这是 OAuth 正常现象，常见原因：

- 用户重复刷新回调页。
- code 已经被使用过一次（OAuth code 只能用一次）。
- 请回到 `/sso/:tenantId` 重新发起授权。

### Q3. 是否支持 SAML 2.0？

A. 当前版本仅支持 OAuth 2.0 流程，SAML 2.0 暂未支持。

### Q4. 用户首次 SSO 登录会自动创建账号吗？

A. 会。EASP 会根据唯一标识（openid/unionid）自动创建对应用户，并分配默认角色。

### Q5. SSO 用户可以同时使用标准密码登录吗？

A. 可以。管理员可以在用户管理页为 SSO 用户绑定邮箱和密码，支持两种登录方式。

---

## 8. 企业微信配置示例

1. 在[企业微信开发平台](https://work.weixin.qq.com/wework_admin/frame#apps)创建自建应用。
2. 获取：
   - CorpID = `client_id`
   - AgentSecret = `client_secret`
3. 设置授权回调域为你的 EASP 域名。
4. 在 EASP 中填写：
   - 提供商：企业微信
   - `client_id`: CorpID
   - `client_secret`: AgentSecret
   - `redirect_uri`: `https://easp.example.com/sso/:tenantId/callback`
   - `authorize_url`: `https://open.weixin.qq.com/connect/oauth2/authorize`
   - `token_url`: `https://qyapi.weixin.qq.com/cgi-bin/gettoken`
   - `userinfo_url`: `https://qyapi.weixin.qq.com/cgi-bin/user/getuserinfo`
   - `scopes`: `snsapi_userinfo`

---

## 9. 钉钉配置示例

1. 在[钉钉开放平台](https://open.dingtalk.com/)创建 H5 微应用。
2. 获取：
   - AppKey = `client_id`
   - AppSecret = `client_secret`
3. 设置回调地址。
4. 在 EASP 中填写：
   - 提供商：钉钉
   - `client_id`: AppKey
   - `client_secret`: AppSecret
   - `redirect_uri`: `https://easp.example.com/sso/:tenantId/callback`
   - `authorize_url`: `https://login.dingtalk.com/oauth2/auth`
   - `token_url`: `https://api.dingtalk.com/v1.0/oauth2/accessToken`
   - `userinfo_url`: `https://api.dingtalk.com/v1.0/contact/users/me`
   - `scopes`: `contact_user`

---

## 10. 安全注意事项

- `client_secret` 是敏感信息，EASP 会加密存储，不会在前端明文展示。
- 确保 `redirect_uri` 的域名是你可控的。
- 生产环境必须使用 HTTPS。
- EASP 会对 state 进行 CSRF 校验，不需要第三方额外处理。
- 如果不需要 SSO，在配置页关闭即可。

---

## 11. 相关接口（开发者参考）

| 接口 | 方法 | 路径 | 说明 |
|---|---|---|---|
| 获取配置 | GET | `/api/v1/tenants/:tenantId/sso/config` | 获取当前租户 SSO 配置（敏感字段脱敏返回） |
| 保存配置 | PUT | `/api/v1/tenants/:tenantId/sso/config` | 保存 SSO 配置 |
| 生成登录 URL | GET | `/api/v1/tenants/:tenantId/sso/login-url` | 生成授权 URL |
| 测试连接 | POST | `/api/v1/tenants/:tenantId/sso/test` | 测试当前配置连通性 |
| 登录回调 | GET | `/sso/:tenantId/callback` | OAuth 回调入口 |
| SSO + 标准登录 | POST | `/api/v1/sso/:tenantId/login` | 统一登录入口，自动回退 |
