import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Switch, Button, Space, Typography, App, Tabs, Alert } from 'antd';
import { SaveOutlined, LinkOutlined, ApiOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title, Text, Paragraph } = Typography;
const { TextArea } = Input;

interface LayoutContext { currentTenant: string; }

interface SSOConfig {
  tenant_id: string;
  enabled: boolean;
  login_url: string;
  login_method: string;
  login_headers: string;
  login_body_template: string;
  user_info_url: string;
  user_info_method: string;
  user_info_headers: string;
  response_mapping: string;
  callback_url: string;
  sync_user_on_login: boolean;
  sync_url: string;
  sync_method: string;
  sync_headers: string;
}

const SSOConfig: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [saving, setSaving] = useState(false);
  const [loginURLs, setLoginURLs] = useState<{ login_url: string; sso_login_url: string } | null>(null);
  const [testing, setTesting] = useState(false);

  const load = async () => {
    if (!currentTenant) return;
    try {
      const res = await client.get(`/tenants/${currentTenant}/sso/config`);
      const data = res.data as SSOConfig;
      form.setFieldsValue({
        enabled: data.enabled || false,
        login_url: data.login_url || '',
        login_method: data.login_method || 'POST',
        login_headers: data.login_headers || '',
        login_body_template: data.login_body_template || '{"username":"{{username}}","password":"{{password}}"}',
        user_info_url: data.user_info_url || '',
        user_info_method: data.user_info_method || 'GET',
        user_info_headers: data.user_info_headers || '',
        response_mapping: data.response_mapping || '{"token":"$.token","user_id":"$.user.id","email":"$.user.email","display_name":"$.user.name"}',
        callback_url: data.callback_url || '',
        sync_user_on_login: data.sync_user_on_login !== false,
        sync_url: data.sync_url || '',
        sync_method: data.sync_method || 'POST',
        sync_headers: data.sync_headers || '',
      });
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    setLoginURLs(null);
    load();
  }, [currentTenant]);

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await client.put(`/tenants/${currentTenant}/sso/config`, values);
      message.success('保存成功');
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const onGenerateURL = async () => {
    try {
      const res = await client.get(`/tenants/${currentTenant}/sso/login-url`);
      setLoginURLs(res.data as { login_url: string; sso_login_url: string });
    } catch {
      message.error('生成链接失败');
    }
  };

  const onTestConnection = async () => {
    setTesting(true);
    try {
      const res = await client.post(`/tenants/${currentTenant}/sso/test`);
      const data = res.data as { success: boolean; message: string };
      if (data.success) {
        message.success('连接成功');
      } else {
        message.error('连接失败');
      }
    } catch {
      message.error('测试失败');
    } finally {
      setTesting(false);
    }
  };

  return (
    <div>
      <Title level={3}>SSO配置</Title>
      
      <Tabs items={[
        {
          key: 'config',
          label: '接口配置',
          children: (
            <Form form={form} layout="vertical">
              <Card title="基础配置" style={{ marginBottom: 16 }}>
                <Form.Item name="enabled" label="启用SSO" valuePropName="checked">
                  <Switch />
                </Form.Item>
              </Card>

              <Card title="业务系统登录接口" style={{ marginBottom: 16 }}>
                <Form.Item name="login_url" label="登录接口地址" rules={[{ required: true, message: '请输入登录接口地址' }]}>
                  <Input placeholder="https://your-business.com/api/login" />
                </Form.Item>
                <Form.Item name="login_method" label="HTTP方法">
                  <Input placeholder="POST" />
                </Form.Item>
                <Form.Item name="login_headers" label="自定义请求头 (JSON)">
                  <TextArea rows={2} placeholder='{"Authorization":"Bearer xxx"}' />
                </Form.Item>
                <Form.Item name="login_body_template" label="请求体模板" extra="支持 {{username}} 和 {{password}} 占位符">
                  <TextArea rows={4} placeholder='{"username":"{{username}}","password":"{{password}}"}' />
                </Form.Item>
                <Button icon={<ApiOutlined />} onClick={onTestConnection} loading={testing}>测试连接</Button>
              </Card>

              <Card title="响应映射配置" style={{ marginBottom: 16 }}>
                <Form.Item name="response_mapping" label="响应字段映射 (JSON)" extra="使用 $.path 格式映射业务系统响应字段">
                  <TextArea rows={6} placeholder='{"token":"$.token","user_id":"$.user.id","email":"$.user.email","display_name":"$.user.name"}' />
                </Form.Item>
              </Card>

              <Card title="用户信息同步 (可选)" style={{ marginBottom: 16 }}>
                <Form.Item name="sync_user_on_login" label="登录时同步用户信息" valuePropName="checked">
                  <Switch />
                </Form.Item>
                <Form.Item name="sync_url" label="同步接口地址">
                  <Input placeholder="https://your-business.com/api/sync-user" />
                </Form.Item>
                <Form.Item name="sync_method" label="HTTP方法">
                  <Input placeholder="POST" />
                </Form.Item>
                <Form.Item name="sync_headers" label="自定义请求头 (JSON)">
                  <TextArea rows={2} placeholder='{"Authorization":"Bearer xxx"}' />
                </Form.Item>
              </Card>

              <Card title="回调配置 (可选)" style={{ marginBottom: 16 }}>
                <Form.Item name="callback_url" label="回调地址" extra="登录成功后回调的URL">
                  <Input placeholder="https://your-app.com/callback" />
                </Form.Item>
              </Card>

              <Button type="primary" icon={<SaveOutlined />} onClick={onSave} loading={saving} size="large">
                保存配置
              </Button>
            </Form>
          )
        },
        {
          key: 'urls',
          label: '登录链接',
          children: (
            <Card>
              <Paragraph>
                生成租户专属的登录链接，用户访问此链接时将使用该租户的SSO配置进行登录。
              </Paragraph>
              <Button type="primary" icon={<LinkOutlined />} onClick={onGenerateURL} style={{ marginBottom: 16 }}>
                生成登录链接
              </Button>
              
              {loginURLs && (
                <Space direction="vertical" style={{ width: '100%' }}>
                  <Card size="small">
                    <Text strong>标准登录页：</Text>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
                      <Text code copyable>{loginURLs.login_url}</Text>
                    </div>
                  </Card>
                  <Card size="small">
                    <Text strong>SSO直接登录：</Text>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 8 }}>
                      <Text code copyable>{loginURLs.sso_login_url}</Text>
                    </div>
                  </Card>
                  <Alert
                    type="info"
                    message="使用说明"
                    description={
                      <ul>
                        <li>标准登录页：显示EASP登录界面，用户输入账号密码后调用SSO</li>
                        <li>SSO直接登录：跳转到业务系统登录页，登录成功后回调EASP</li>
                      </ul>
                    }
                  />
                </Space>
              )}
            </Card>
          )
        },
        {
          key: 'guide',
          label: '接入指南',
          children: (
            <Card>
              <Title level={4}>SSO接入指南</Title>
              
              <Title level={5}>1. 业务系统登录接口规范</Title>
              <Paragraph>
                业务系统需要提供一个登录接口，接收以下格式的请求：
              </Paragraph>
              <Card size="small" style={{ marginBottom: 16 }}>
                <Text code>POST /api/login</Text>
                <pre>{`{
  "username": "user@example.com",
  "password": "password123"
}`}</pre>
              </Card>
              
              <Paragraph>
                返回格式示例：
              </Paragraph>
              <Card size="small" style={{ marginBottom: 16 }}>
                <pre>{`{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "12345",
    "email": "user@example.com",
    "name": "张三"
  }
}`}</pre>
              </Card>

              <Title level={5}>2. 响应映射配置</Title>
              <Paragraph>
                使用JSONPath格式映射响应字段：
              </Paragraph>
              <Card size="small" style={{ marginBottom: 16 }}>
                <pre>{`{
  "token": "$.token",           // 业务系统的token
  "user_id": "$.user.id",       // 用户ID
  "email": "$.user.email",      // 用户邮箱
  "display_name": "$.user.name" // 用户名称
}`}</pre>
              </Card>

              <Title level={5}>3. 同步接口规范 (可选)</Title>
              <Paragraph>
                如果需要在用户登录时同步信息到业务系统，配置同步接口：
              </Paragraph>
              <Card size="small" style={{ marginBottom: 16 }}>
                <Text code>POST /api/sync-user</Text>
                <pre>{`{
  "easp_user_id": "uuid",
  "email": "user@example.com",
  "display_name": "张三",
  "biz_token": "业务系统token"
}`}</pre>
              </Card>

              <Title level={5}>4. 登录流程</Title>
              <ol>
                <li>用户访问租户专属登录链接</li>
                <li>输入账号密码</li>
                <li>EASP调用业务系统登录接口</li>
                <li>验证成功后，EASP创建/更新本地用户</li>
                <li>生成EASP的JWT token</li>
                <li>返回token给前端</li>
              </ol>
            </Card>
          )
        }
      ]} />
    </div>
  );
};

export default SSOConfig;
