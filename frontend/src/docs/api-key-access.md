# API Key 接入文档

> 本文档用于外部系统接入 EASP Embed API。页面为公开文档页，不依赖登录态和功能权限，便于复制给小程序、APP、H5 或第三方系统开发人员。

## 1. 接入概览

EASP API Key 用于让外部系统在不要求终端用户登录 EASP 的情况下，调用嵌入式 AI 助手能力。

典型场景：

- 小程序客服助手
- APP 内嵌 AI 问答
- H5 活动页智能咨询
- 第三方业务系统插件

基础调用流程：

1. 在 EASP「API Key 管理」页面创建 API Key。
2. 后端服务保存 API Key，**不要放在前端、小程序或 App 客户端源码中**。
3. 外部系统后端调用 Embed API。
4. Embed API 返回 SSE 流式响应或会话数据。

## 2. 安全要求

- API Key 等同于系统访问凭证，只会在创建时显示一次。
- 不要把 API Key 写入浏览器前端、小程序端、App 包体或公开仓库。
- 推荐由业务后端代转请求，再返回给客户端。
- 如怀疑泄露，请立即在 EASP 管理页禁用或删除旧 Key，并创建新 Key。

## 3. 认证方式

Embed API 支持 Bearer Token 方式：

```http
Authorization: Bearer easp_xxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json
```

所有 Embed API 当前基础路径：

```text
/embed/v1
```

示例中用 `{BASE_URL}` 表示 EASP 平台访问地址，例如：

```text
http://8.130.48.99:8080
```

## 4. 权限范围 Scopes

创建 API Key 时可选择权限范围：

| Scope | 说明 |
| --- | --- |
| `chat` | 允许调用聊天接口 `/embed/v1/chat` |
| `sessions` | 允许创建、查询会话与消息 |
| 留空 | 表示全部权限 |

如果接口返回 `403`，请检查 API Key 是否包含对应 scope。

## 5. 发送聊天消息

接口：

```http
POST /embed/v1/chat
```

请求体：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message` | string | 是 | 用户发送的问题或指令 |
| `session_id` | string | 否 | 已有会话 ID；不传则自动创建新会话 |
| `visitor_id` | string | 否 | 外部访客 ID；不传默认为 `anonymous` |
| `context` | object | 否 | 业务上下文，例如订单、页面、用户标签等 |

curl 示例：

```bash
curl -N -X POST "{BASE_URL}/embed/v1/chat" \
  -H "Authorization: Bearer {API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "visitor_id": "user_10001",
    "message": "帮我介绍一下这个产品",
    "context": {
      "source": "miniapp",
      "page": "product_detail",
      "product_id": "prod_001"
    }
  }'
```

返回格式为 SSE：

```text
event: delta
data: {"content":"您好，我可以..."}

event: done
data: null
```

前端消费 SSE 时，注意该接口是 `POST` 流式响应，不是浏览器原生 `EventSource` 的 GET 模式。推荐用 `fetch` 读取 `ReadableStream`。

## 6. 创建会话

接口：

```http
POST /embed/v1/sessions
```

需要 `sessions` scope。

请求体：

```json
{
  "visitor_id": "user_10001",
  "metadata": {
    "source": "miniapp",
    "nickname": "张三"
  }
}
```

返回示例：

```json
{
  "id": "session-uuid",
  "visitor_id": "user_10001",
  "created_at": "2026-06-13T10:00:00Z"
}
```

## 7. 查询会话列表

接口：

```http
GET /embed/v1/sessions
GET /embed/v1/sessions?visitor_id=user_10001
```

需要 `sessions` scope。

curl 示例：

```bash
curl "{BASE_URL}/embed/v1/sessions?visitor_id=user_10001" \
  -H "Authorization: Bearer {API_KEY}"
```

## 8. 查询会话消息

接口：

```http
GET /embed/v1/sessions/{session_id}/messages
```

需要 `sessions` scope。

curl 示例：

```bash
curl "{BASE_URL}/embed/v1/sessions/{SESSION_ID}/messages" \
  -H "Authorization: Bearer {API_KEY}"
```

## 9. JavaScript 调用示例

```ts
async function chatWithEasp(params: {
  baseUrl: string;
  apiKey: string;
  message: string;
  visitorId?: string;
  sessionId?: string;
  context?: Record<string, unknown>;
  onDelta?: (text: string) => void;
}) {
  const res = await fetch(`${params.baseUrl}/embed/v1/chat`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${params.apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      message: params.message,
      visitor_id: params.visitorId,
      session_id: params.sessionId,
      context: params.context,
    }),
  });

  if (!res.ok || !res.body) {
    throw new Error(`EASP request failed: ${res.status}`);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder('utf-8');
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    const events = buffer.split('\n\n');
    buffer = events.pop() || '';

    for (const event of events) {
      const dataLine = event.split('\n').find(line => line.startsWith('data:'));
      if (!dataLine) continue;
      const raw = dataLine.replace(/^data:\s*/, '');
      if (!raw || raw === 'null') continue;
      const payload = JSON.parse(raw);
      if (payload.content) params.onDelta?.(payload.content);
    }
  }
}
```

## 10. 常见错误

| 状态码 | 可能原因 | 处理方式 |
| --- | --- | --- |
| `401` | 未传 API Key、格式错误、Key 已删除或不存在 | 检查 `Authorization: Bearer {API_KEY}` |
| `403` | Scope 不足或 Key 被禁用 | 调整权限范围或启用 Key |
| `404` | 会话不存在或不属于当前租户 | 检查 `session_id` |
| `429` | 触发限流或配额控制 | 降低调用频率或检查套餐/配额 |
| `500` | 模型配置异常或服务内部错误 | 检查租户模型配置与服务日志 |

## 11. 最佳实践

- 一个业务系统或应用端建议单独创建一个 API Key，便于审计和停用。
- Key 名称建议包含用途，例如：`小程序客服生产环境`、`H5活动页测试环境`。
- 生产和测试环境使用不同 API Key。
- 上线前先用较小 scope 测试，确认需要会话接口时再增加 `sessions`。
- 对接端记录 `session_id`，便于用户继续同一轮对话。

## 12. 后续扩展预留

该文档页当前从 Markdown 源文件加载，后续可以继续扩展：

- SDK 示例：小程序、H5、Node.js、Java、Go
- OpenAPI Schema
- 嵌入式聊天 UI 组件
- 错误码明细
- 多租户接入规范
