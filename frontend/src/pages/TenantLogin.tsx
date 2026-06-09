import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Typography, App, Tabs, Result } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from 'react-router-dom';
import { authApi } from '../api/auth';
import { useAuth } from '../contexts/AuthContext';
import client from '../api/client';

const { Title, Text } = Typography;

const TenantLogin: React.FC = () => {
  const { tenantId } = useParams<{ tenantId: string }>();
  const navigate = useNavigate();
  const { message } = App.useApp();
  const { reloadUser } = useAuth();
  const [loginForm] = Form.useForm();
  const [registerForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('login');
  const [tenantName, setTenantName] = useState<string>('');
  const [tenantError, setTenantError] = useState<string>('');
  const [checking, setChecking] = useState(true);

  // 验证租户是否存在
  useEffect(() => {
    if (!tenantId) {
      setTenantError('缺少租户ID参数');
      setChecking(false);
      return;
    }

    setChecking(true);
    client.get(`/tenants/${tenantId}`).then(res => {
      const data = res.data as { name: string };
      setTenantName(data.name);
      setTenantError('');
    }).catch(() => {
      setTenantError('租户不存在，请检查租户ID是否正确');
    }).finally(() => {
      setChecking(false);
    });
  }, [tenantId]);

  // 登录 — 统一调 /sso/:tenantId/login，后端自动判断走标准登录或SSO
  const onLogin = async (values: { username: string; password: string }) => {
    setLoading(true);
    try {
      const res = await client.post(`/sso/${tenantId}/login`, values);
      const data = res.data as {
        user: { id: string; email: string; display_name: string; is_admin?: boolean };
        tokens: { access_token: string; refresh_token: string };
        callback_url?: string;
        biz_token?: string;
      };

      localStorage.setItem('access_token', data.tokens.access_token);
      localStorage.setItem('refresh_token', data.tokens.refresh_token);

      // 等 AuthContext 加载用户后再跳转
      await reloadUser();
      message.success('登录成功');

      if (data.callback_url) {
        window.location.href = `${data.callback_url}?token=${data.tokens.access_token}&biz_token=${data.biz_token || ''}`;
      } else {
        navigate('/', { replace: true });
      }
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '登录失败');
    } finally {
      setLoading(false);
    }
  };

  // 注册
  const onRegister = async (values: { email: string; password: string; display_name?: string }) => {
    setLoading(true);
    try {
      await authApi.register({ ...values, tenant_id: tenantId! });
      message.success('注册成功，请登录');
      setActiveTab('login');
      registerForm.resetFields();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '注册失败');
    } finally {
      setLoading(false);
    }
  };

  // 加载中
  if (checking) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center', background: '#f0f2f5' }}>
        <Card style={{ width: 420, textAlign: 'center' }}>
          <Text type="secondary">正在验证租户信息...</Text>
        </Card>
      </div>
    );
  }

  // 租户不存在
  if (tenantError) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center', background: '#f0f2f5' }}>
        <Card style={{ width: 420 }}>
          <Result
            status="warning"
            title="租户访问失败"
            subTitle={tenantError}
            extra={
              <Button type="primary" onClick={() => navigate('/login', { replace: true })}>
                返回标准登录
              </Button>
            }
          />
        </Card>
      </div>
    );
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center', background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)' }}>
      <Card style={{ width: 420, borderRadius: 12 }} bodyStyle={{ padding: '40px 32px' }}>
        <div style={{ textAlign: 'center', marginBottom: 32 }}>
          <Title level={2} style={{ margin: 0 }}>{tenantName}</Title>
          <Text type="secondary">EASP Platform</Text>
        </div>

        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: 'login',
            label: '登录',
            children: (
              <Form form={loginForm} onFinish={onLogin} size="large">
                <Form.Item name="username" rules={[{ required: true, message: '请输入邮箱' }]}>
                  <Input prefix={<UserOutlined />} placeholder="邮箱" />
                </Form.Item>
                <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
                  <Input.Password prefix={<LockOutlined />} placeholder="密码" />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} block>登录</Button>
                </Form.Item>
              </Form>
            ),
          },
          {
            key: 'register',
            label: '注册',
            children: (
              <Form form={registerForm} onFinish={onRegister} size="large" initialValues={{ tenant_id: tenantId }}>
                <Form.Item name="tenant_id" label="所属租户">
                  <Input disabled />
                </Form.Item>
                <Form.Item name="email" rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]}>
                  <Input prefix={<UserOutlined />} placeholder="邮箱" />
                </Form.Item>
                <Form.Item name="display_name">
                  <Input placeholder="显示名称（可选）" />
                </Form.Item>
                <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }, { min: 6, message: '密码至少6位' }]}>
                  <Input.Password prefix={<LockOutlined />} placeholder="密码" />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} block>注册</Button>
                </Form.Item>
              </Form>
            ),
          },
        ]} />

        <div style={{ textAlign: 'center', marginTop: 16 }}>
          <a href="/login">返回标准登录</a>
        </div>
      </Card>
    </div>
  );
};

export default TenantLogin;
