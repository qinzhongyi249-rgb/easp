import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Switch, Button, Space, Typography, App, Alert } from 'antd';
import { SaveOutlined, LinkOutlined, KeyOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title, Text } = Typography;
const { TextArea } = Input;

interface LayoutContext { currentTenant: string; }

const SSOConfigPage: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [saving, setSaving] = useState(false);
  const [form] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    if (!currentTenant) return;
    try {
      const res = await client.get(`/tenants/${currentTenant}/sso-config`);
      form.setFieldsValue(res.data);
    } catch {
      // 配置不存在
    }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onSave = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      await client.put(`/tenants/${currentTenant}/sso-config`, values);
      message.success('保存成功');
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const copyLink = () => {
    const link = `${window.location.origin}/sso/${currentTenant}`;
    navigator.clipboard.writeText(link);
    message.success('链接已复制');
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><KeyOutlined /> SSO配置</Title>
        <Space>
          <Button icon={<LinkOutlined />} onClick={copyLink}>复制登录链接</Button>
          <Button type="primary" icon={<SaveOutlined />} onClick={onSave} loading={saving}>保存</Button>
        </Space>
      </div>

      <Alert
        message="SSO登录链接"
        description={
          <Space>
            <Text copyable>{`${window.location.origin}/sso/${currentTenant}`}</Text>
          </Space>
        }
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
      />

      <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
        <Card title="基本设置" style={{ marginBottom: 16 }}>
          <Form.Item name="enabled" label="启用SSO" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="login_url" label="登录URL" rules={[{ required: true }]}>
            <Input placeholder="https://sso.example.com/login" />
          </Form.Item>
          <Form.Item name="login_method" label="请求方法">
            <Input placeholder="POST" />
          </Form.Item>
          <Form.Item name="login_headers" label="请求头(JSON)">
            <TextArea rows={2} placeholder='{"Content-Type": "application/x-www-form-urlencoded"}' />
          </Form.Item>
          <Form.Item name="login_body_template" label="请求体模板">
            <TextArea rows={3} placeholder="username={username}&password={password}" />
          </Form.Item>
        </Card>

        <Card title="用户信息">
          <Form.Item name="user_info_url" label="用户信息URL">
            <Input placeholder="https://sso.example.com/userinfo" />
          </Form.Item>
          <Form.Item name="user_info_method" label="请求方法">
            <Input placeholder="GET" />
          </Form.Item>
          <Form.Item name="user_info_path" label="用户信息路径">
            <Input placeholder="data.user" />
          </Form.Item>
          <Form.Item name="field_mapping" label="字段映射(JSON)">
            <TextArea rows={3} placeholder='{"email": "email", "name": "display_name"}' />
          </Form.Item>
        </Card>
      </Form>
    </div>
  );
};

export default SSOConfigPage;
