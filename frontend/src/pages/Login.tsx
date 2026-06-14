import React, { useState } from 'react';
import { Card, Form, Input, Button, Typography, Tabs, App } from 'antd';
import { UserOutlined, LockOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { authApi } from '../api/auth';

const { Title, Text } = Typography;

const Login: React.FC = () => {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('login');
  const { message } = App.useApp();
  const isMobile = window.innerWidth < 768;

  const onLogin = async (values: { identifier: string; password: string; tenant_id?: string }) => {
    setLoading(true);
    try {
      await login(values.identifier, values.password, values.tenant_id);
      message.success('登录成功');
      navigate('/dashboard', { replace: true });
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '账号或密码错误');
    } finally {
      setLoading(false);
    }
  };

  const onRegister = async (values: { tenant_id: string; email?: string; phone?: string; password: string; display_name: string }) => {
    setLoading(true);
    try {
      await authApi.register(values);
      message.success('注册成功，请登录');
      setActiveTab('login');
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '注册失败');
    } finally {
      setLoading(false);
    }
  };

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
          <Title level={isMobile ? 3 : 2} style={{ margin: 0 }}>EASP Platform</Title>
          <Text type="secondary">企业API转MCP服务平台</Text>
        </div>
        <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
          {
            key: 'login',
            label: '登录',
            children: (
              <Form onFinish={onLogin} size={isMobile ? 'middle' : 'large'}>
                <Form.Item name="tenant_id">
                  <Input placeholder="租户号（选填）" />
                </Form.Item>
                <Form.Item name="identifier" rules={[{ required: true, message: '请输入邮箱或手机号' }]}>
                  <Input prefix={<UserOutlined />} placeholder="邮箱 / 手机号" />
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
              <Form onFinish={onRegister} size={isMobile ? 'middle' : 'large'}>
                <Form.Item name="tenant_id" rules={[{ required: true, message: '请输入租户ID' }]}>
                  <Input placeholder="租户ID" />
                </Form.Item>
                <Form.Item name="email" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
                  <Input prefix={<UserOutlined />} placeholder="邮箱" />
                </Form.Item>
                <Form.Item name="phone">
                  <Input placeholder="手机号（邮箱或手机号至少填一个）" />
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
