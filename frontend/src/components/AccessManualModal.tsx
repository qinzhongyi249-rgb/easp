import React from 'react';
import { Alert, Button, Modal, Space, Tabs, Typography } from 'antd';
import { CopyOutlined, FileTextOutlined } from '@ant-design/icons';

const { Paragraph, Text } = Typography;

type ManualType = 'user' | 'assistant';

interface AccessManualModalProps {
  type: ManualType;
  open: boolean;
  tenantId: string;
  onClose: () => void;
}

interface Snippet {
  key: string;
  title: string;
  description: string;
  language: string;
  code: string;
}

const originOf = () => {
  if (typeof window === 'undefined') return 'https://easp.example.com';
  return window.location.origin;
};

const buildUserSnippets = (baseUrl: string, tenantId: string): Snippet[] => [
  {
    key: 'server-sync',
    title: '服务端同步外部用户',
    description: '业务系统后端使用嵌入应用 AppID + App Secret 签名同步用户；不使用 EASP 后台 JWT。',
    language: 'javascript',
    code: `import crypto from 'node:crypto';

const EASP_BASE_URL = '${baseUrl}';
const TENANT_ID = '${tenantId}';
const APP_ID = process.env.EASP_APP_ID;          // 嵌入接入应用 app_id
const APP_SECRET = process.env.EASP_APP_SECRET;  // 只允许保存在业务系统后端
const EXTERNAL_SYSTEM = 'crm';

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
  ].map(([k, v]) => \`${'${k}'}=${'${v}'}\`).join('&');
  return crypto.createHmac('sha256', secretHash).update(payload).digest('hex');
}

const body = JSON.stringify({
  tenant_id: TENANT_ID,
  external_system: EXTERNAL_SYSTEM,
  batch_id: 'init-20260622-001',
  mode: 'init',
  // 如果要让导入用户可登录 EASP 控制台：必须提供 account，并配置 default_password 或 users[].password；email/phone 只是属性
  default_password: 'Init@123456',
  users: [{
    external_user_id: 'u_10001',       // 必传：外部系统稳定用户 ID
    account: 'zhangsan',             // 登录账号：租户内唯一；不传则默认 external_user_id
    user_uid: 'crm:u_10001',           // 建议：EASP 内稳定用户唯一标识
    display_name: '张三',
    email: 'zhangsan@example.com',     // 属性信息：不作为登录账号、不唯一
    phone: '13800000000',              // 属性信息：不作为登录账号、不唯一
    department: '销售部',
    position: '客户经理',
    profile: { level: 'L2' },
    attributes: { region: '华北' },
    identities: [
      { provider: 'wechat', provider_user_id: 'wx_openid_xxx', display_name: '张三' }
    ]
  }]
});

const timestamp = Math.floor(Date.now() / 1000).toString();
const nonce = crypto.randomUUID();
const signature = sign({ appId: APP_ID, appSecret: APP_SECRET, timestamp, nonce, tenantId: TENANT_ID, externalSystem: EXTERNAL_SYSTEM, body });

const res = await fetch(\`${'${EASP_BASE_URL}'}/api/v1/embed/users/sync\`, {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-EASP-App-Id': APP_ID,
    'X-EASP-Timestamp': timestamp,
    'X-EASP-Nonce': nonce,
    'X-EASP-Signature': signature,
  },
  body,
});
console.log(await res.json());`,
  },
  {
    key: 'curl-shape',
    title: 'HTTP 请求结构',
    description: '签名由业务系统后端生成。此示例展示最终请求形态，不能把 App Secret 放到浏览器。',
    language: 'bash',
    code: `curl -X POST '${baseUrl}/api/v1/embed/users/sync' \\
  -H 'Content-Type: application/json' \\
  -H 'X-EASP-App-Id: <app_id>' \\
  -H 'X-EASP-Timestamp: <unix_seconds>' \\
  -H 'X-EASP-Nonce: <random_nonce>' \\
  -H 'X-EASP-Signature: <hmac_sha256_signature>' \\
  -d '{
    "tenant_id": "${tenantId}",
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
        "identities": [
          { "provider": "wechat", "provider_user_id": "wx_openid_xxx" }
        ]
      }
    ]
  }'`,
  },
  {
    key: 'idempotency',
    title: '重复性校验规则',
    description: 'EASP 后端会做幂等和冲突判断，业务系统可安全重试初始化批次。',
    language: 'text',
    code: `后端校验顺序：
1. tenant_id 必须存在、active、未到期，且不能超过租户用户上限。
2. app_id 必须属于该 tenant_id + external_system，且状态 active。
3. 签名必须通过，时间戳默认 5 分钟有效。
4. 先按 external_system + external_user_id 查已有绑定。
5. 再按 user_uid / email / phone / identities(provider + provider_user_id) 查既有 EASP 用户。
6. 命中同一个用户：幂等更新并补齐 external_user_bindings / user_identity_bindings。
7. 命中多个不同用户：返回 conflict，避免误绑定。
8. role_ids 只能绑定当前租户角色，禁止系统角色。

登录账号与密码规则：
- 如需账号密码登录 EASP，必须提供 account；登录页使用 account + 密码。
- account 租户内唯一，不随 email/phone 变更；email/phone 只是属性信息，不参与唯一性校验。
- 如果不传 account，EASP 默认使用 external_user_id 作为账号。
- default_password 可作为本批次默认密码；users[].password 可覆盖单个用户密码，二者都要求至少 6 位。
- 如果不配置密码，EASP 会生成不可见随机密码，用户只能用于嵌入助手身份映射，后续需在用户管理页重置密码后才能登录。`,
  },
];

const buildAssistantSnippets = (baseUrl: string, tenantId: string): Snippet[] => [
  {
    key: 'token-exchange',
    title: '服务端换取 easp-api-token',
    description: '业务系统后端生成签名并换取嵌入式助手专用 token；如助手需要代用户访问业务系统，可同时传 external_access_token。',
    language: 'bash',
    code: `curl -X POST '${baseUrl}/api/v1/embed/token/exchange' \\
  -H 'Content-Type: application/json' \\
  -H 'X-EASP-App-Id: <app_id>' \\
  -H 'X-EASP-Timestamp: <unix_seconds>' \\
  -H 'X-EASP-Nonce: <random_nonce>' \\
  -H 'X-EASP-Signature: <hmac_sha256_signature>' \\
  -d '{
    "tenant_id": "${tenantId}",
    "external_system": "crm",
    "external_user_id": "u_10001",
    "external_access_token": "<current_business_user_token>",
    "external_token_expires_at": 1790000000
  }'`,
  },
  {
    key: 'sdk',
    title: 'JS SDK 嵌入',
    description: '适合 H5/PC 页面。前端只拿业务系统后端返回的 easp-api-token，不接触 App Secret。',
    language: 'html',
    code: `<script src="${baseUrl}/embed/assistant.js"></script>
<script>
  EASPAssistant.init({
    baseUrl: '${baseUrl}',
    tenantId: '${tenantId}',
    tokenProvider: async () => {
      const res = await fetch('/api/easp-assistant-token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ external_user_id: 'u_10001' })
      });
      const data = await res.json();
      return data.easp_api_token;
    },
    user: {
      external_system: 'crm',
      external_user_id: 'u_10001',
      display_name: '张三'
    }
  });
</script>`,
  },
  {
    key: 'iframe',
    title: 'iframe 嵌入',
    description: '适合快速嵌入固定浮窗。token 仍建议由业务系统后端签发后传入。',
    language: 'html',
    code: `<iframe
  id="easp-assistant"
  src="${baseUrl}/embed/assistant-frame.html?tenant_id=${tenantId}"
  style="position:fixed;right:20px;bottom:20px;width:420px;height:680px;border:0;z-index:99999;"
></iframe>`,
  },
];

const manualConfig = {
  user: {
    title: '用户接入手册',
    tip: '以下代码已按当前租户生成。用户同步是业务系统后端的初始化/异步动作，使用嵌入应用签名，不使用 EASP 后台 JWT。',
    fullDocUrl: '/docs/user-access.html',
    markdownUrl: '/docs/user-access.md',
    build: buildUserSnippets,
  },
  assistant: {
    title: 'AI助手接入手册',
    tip: '以下代码已按当前租户生成。用于第三方业务系统嵌入 EASP AI 助手。',
    fullDocUrl: '/docs/ai-assistant-access.html',
    markdownUrl: '/docs/ai-assistant-access.md',
    build: buildAssistantSnippets,
  },
} satisfies Record<ManualType, {
  title: string;
  tip: string;
  fullDocUrl: string;
  markdownUrl: string;
  build: (baseUrl: string, tenantId: string) => Snippet[];
}>;

const CodeBlock: React.FC<{ snippet: Snippet }> = ({ snippet }) => {
  const copy = async () => navigator.clipboard.writeText(snippet.code);
  return (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 8 }} align="start">
        <div>
          <Text strong>{snippet.title}</Text>
          <div><Text type="secondary">{snippet.description}</Text></div>
        </div>
        <Button icon={<CopyOutlined />} onClick={copy}>复制代码</Button>
      </Space>
      <pre style={{
        margin: 0,
        padding: 12,
        borderRadius: 8,
        background: '#0f172a',
        color: '#e2e8f0',
        overflow: 'auto',
        fontSize: 12,
        lineHeight: 1.6,
      }}><code>{snippet.code}</code></pre>
    </div>
  );
};

const AccessManualModal: React.FC<AccessManualModalProps> = ({ type, open, tenantId, onClose }) => {
  const cfg = manualConfig[type];
  const baseUrl = originOf();
  const snippets = cfg.build(baseUrl, tenantId || '<tenant_id>');

  return (
    <Modal
      title={cfg.title}
      open={open}
      onCancel={onClose}
      width={900}
      footer={[
        <Button key="md" icon={<FileTextOutlined />} href={cfg.markdownUrl} target="_blank">Markdown</Button>,
        <Button key="full" type="primary" icon={<FileTextOutlined />} href={cfg.fullDocUrl} target="_blank">查看完整文档</Button>,
      ]}
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert type="info" showIcon message={cfg.tip} description={tenantId ? `当前租户 ID：${tenantId}` : '未选择租户，请先在左上角选择租户。'} />
        <Tabs
          items={snippets.map(snippet => ({
            key: snippet.key,
            label: snippet.title,
            children: <CodeBlock snippet={snippet} />,
          }))}
        />
        <Paragraph type="secondary" style={{ marginBottom: 0 }}>
          安全边界：App Secret 只能保存在业务系统后端；H5/PC 前端只能使用后端换回的 <Text code>easp-api-token</Text>。
        </Paragraph>
      </Space>
    </Modal>
  );
};

export default AccessManualModal;
