import React, { useEffect, useState } from 'react';
import { Card, Form, Input, Button, Typography, Tabs, App, Result } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { authApi, type AuthResponse } from '../api/auth';
import client from '../api/client';

const { Title, Text } = Typography;

type LoginValues = {
  identifier: string;
  password: string;
  tenant_id?: string;
};

type RegisterValues = {
  tenant_id: string;
  account: string;
  email?: string;
  phone?: string;
  password: string;
  display_name: string;
};

type TenantLoginResponse = AuthResponse & {
  callback_url?: string;
  biz_token?: string;
};

const Login: React.FC = () => {
  const { tenantId } = useParams<{ tenantId: string }>();
  const isTenantEntry = Boolean(tenantId);
  const { login, reloadUser } = useAuth();
  const navigate = useNavigate();
  const [loginForm] = Form.useForm<LoginValues>();
  const [registerForm] = Form.useForm<RegisterValues>();
  const [loading, setLoading] = useState(false);
  const [checkingTenant, setCheckingTenant] = useState(false);
  const [tenantName, setTenantName] = useState('');
  const [tenantError, setTenantError] = useState('');
  const [activeTab, setActiveTab] = useState('login');
  const { message } = App.useApp();
  const isMobile = window.innerWidth < 768;

  useEffect(() => {
    if (!tenantId) {
      setTenantName('');
      setTenantError('');
      setCheckingTenant(false);
      loginForm.resetFields(['tenant_id']);
      registerForm.resetFields(['tenant_id']);
      return;
    }

    loginForm.setFieldsValue({ tenant_id: tenantId });
    registerForm.setFieldsValue({ tenant_id: tenantId });
    setCheckingTenant(true);
    client.get(`/tenants/${tenantId}`).then(res => {
      const data = res.data as { name?: string };
      setTenantName(data.name || tenantId);
      setTenantError('');
    }).catch(() => {
      setTenantError('租户不存在，请检查租户号是否正确');
    }).finally(() => {
      setCheckingTenant(false);
    });
  }, [tenantId, loginForm, registerForm]);

  const onLogin = async (values: LoginValues) => {
    setLoading(true);
    try {
      if (tenantId) {
        const res = await client.post<TenantLoginResponse>(`/sso/${tenantId}/login`, {
          username: values.identifier,
          password: values.password,
        });
        const data = res.data;
        localStorage.setItem('access_token', data.tokens.access_token);
        localStorage.setItem('refresh_token', data.tokens.refresh_token);
        await reloadUser();
        message.success('登录成功');

        if (data.callback_url) {
          window.location.href = `${data.callback_url}?token=${data.tokens.access_token}&biz_token=${data.biz_token || ''}`;
          return;
        }
        navigate('/admin/dashboard', { replace: true });
        return;
      }

      await login(values.identifier, values.password, values.tenant_id);
      message.success('登录成功');
      navigate('/admin/dashboard', { replace: true });
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '账号或密码错误');
    } finally {
      setLoading(false);
    }
  };

  const onRegister = async (values: RegisterValues) => {
    setLoading(true);
    try {
      await authApi.register({
        ...values,
        tenant_id: tenantId || values.tenant_id,
      });
      message.success('注册成功，请登录');
      setActiveTab('login');
      registerForm.resetFields();
      if (tenantId) {
        registerForm.setFieldsValue({ tenant_id: tenantId });
      }
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '注册失败');
    } finally {
      setLoading(false);
    }
  };

  if (checkingTenant) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center', background: '#f0f2f5', padding: isMobile ? 16 : 24 }}>
        <Card style={{ width: isMobile ? '100%' : 420, maxWidth: 420, textAlign: 'center' }}>
          <Text type="secondary">正在验证租户信息...</Text>
        </Card>
      </div>
    );
  }

  if (tenantError) {
    return (
      <div style={{ minHeight: '100vh', display: 'flex', justifyContent: 'center', alignItems: 'center', background: '#f0f2f5', padding: isMobile ? 16 : 24 }}>
        <Card style={{ width: isMobile ? '100%' : 420, maxWidth: 420 }}>
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
    <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      minHeight: '100vh',
      background: 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
      padding: isMobile ? 16 : 24,
    }}>
      <Card style={{
        width: isMobile ? '100%' : 420,
        borderRadius: 12,
        maxWidth: 420,
      }} bodyStyle={{ padding: isMobile ? '24px 16px' : '40px 32px' }}>
        <div style={{ textAlign: 'center', marginBottom: isMobile ? 24 : 32 }}>
          <Title level={isMobile ? 3 : 2} style={{ margin: 0 }}>{tenantName || 'EASP Platform'}</Title>
          <Text type="secondary">{isTenantEntry ? 'EASP Platform' : '企业API转MCP服务平台'}</Text>
        </div>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: 'login',
            label: '登录',
            children: (
              <Form form={loginForm} onFinish={onLogin} size={isMobile ? 'middle' : 'large'} initialValues={{ tenant_id: tenantId }}>
                <Form.Item name="tenant_id">
                  <Input placeholder="租户号（选填）" disabled={isTenantEntry} />
                </Form.Item>
                <Form.Item name="identifier" rules={[{ required: true, message: '请输入账号' }]}>
                  <Input prefix={<UserOutlined />} placeholder="账号" />
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
              <Form form={registerForm} onFinish={onRegister} size={isMobile ? 'middle' : 'large'} initialValues={{ tenant_id: tenantId }}>
                <Form.Item name="tenant_id" rules={[{ required: true, message: '请输入租户号' }]}>
                  <Input placeholder="租户号" disabled={isTenantEntry} />
                </Form.Item>
                <Form.Item name="account" rules={[{ required: true, message: '请输入账号' }]}>
                  <Input prefix={<UserOutlined />} placeholder="登录账号（租户内唯一）" />
                </Form.Item>
                <Form.Item name="email" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
                  <Input placeholder="邮箱（属性信息，可后续修改）" />
                </Form.Item>
                <Form.Item name="phone">
                  <Input placeholder="手机号（属性信息，可后续修改）" />
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
      </Card>
    </div>
  );
};

export default Login;
