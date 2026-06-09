import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Switch, Select, Space, Typography, Popconfirm, Tag, App } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, StarOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { ModelProvider, ModelConfig as MC } from '../api/modelConfig';
import { modelConfigApi } from '../api/modelConfig';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

const ModelConfigPage: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [providers, setProviders] = useState<ModelProvider[]>([]);
  const [configs, setConfigs] = useState<MC[]>([]);
  const [loading, setLoading] = useState(false);
  const [providerModalOpen, setProviderModalOpen] = useState(false);
  const [configModalOpen, setConfigModalOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<ModelProvider | null>(null);
  const [editingConfig, setEditingConfig] = useState<MC | null>(null);
  const [providerForm] = Form.useForm();
  const [configForm] = Form.useForm();

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const [p, c] = await Promise.all([
        modelConfigApi.listProviders(currentTenant),
        modelConfigApi.listConfigs(currentTenant),
      ]);
      setProviders(p.data || []);
      setConfigs(c.data || []);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onProviderOk = async () => {
    const values = await providerForm.validateFields();
    try {
      if (editingProvider) {
        await modelConfigApi.updateProvider(currentTenant, editingProvider.id, values);
        message.success('更新成功');
      } else {
        await modelConfigApi.createProvider(currentTenant, values);
        message.success('创建成功');
      }
      setProviderModalOpen(false); providerForm.resetFields(); setEditingProvider(null); load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '操作失败');
    }
  };

  const onConfigOk = async () => {
    const values = await configForm.validateFields();
    try {
      if (editingConfig) {
        await modelConfigApi.updateConfig(currentTenant, editingConfig.id, values);
        message.success('更新成功');
      } else {
        await modelConfigApi.createConfig(currentTenant, values);
        message.success('创建成功');
      }
      setConfigModalOpen(false); configForm.resetFields(); setEditingConfig(null); load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '操作失败');
    }
  };

  const onSetDefault = async (id: string) => {
    try { await modelConfigApi.setDefault(currentTenant, id); message.success('设为默认成功'); load(); }
    catch { message.error('操作失败'); }
  };

  // 快捷切换供应商启用状态
  const onToggleProvider = async (record: ModelProvider, checked: boolean) => {
    try {
      await modelConfigApi.updateProvider(currentTenant, record.id, { enabled: checked });
      message.success(checked ? '已启用' : '已禁用');
      load();
    } catch { message.error('操作失败'); }
  };

  // 快捷切换模型配置启用状态
  const onToggleConfig = async (record: MC, checked: boolean) => {
    try {
      await modelConfigApi.updateConfig(currentTenant, record.id, { enabled: checked });
      message.success(checked ? '已启用' : '已禁用');
      load();
    } catch { message.error('操作失败'); }
  };

  const providerColumns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, r: ModelProvider) => (
      <Space>
        <span>{v}</span>
        {r.display_name && <Text type="secondary" style={{ fontSize: 12 }}>({r.display_name})</Text>}
      </Space>
    )},
    { title: '类型', dataIndex: 'type', key: 'type', render: (v: string) => <Tag>{v}</Tag> },
    { title: 'Base URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
    { title: '启用', dataIndex: 'enabled', key: 'enabled', width: 80, render: (v: boolean, r: ModelProvider) => (
      <Switch size="small" checked={v !== false} onChange={(checked) => onToggleProvider(r, checked)} />
    )},
    { title: '操作', key: 'action', width: 160, render: (_: unknown, record: ModelProvider) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => {
          setEditingProvider(record);
          providerForm.setFieldsValue(record);
          setProviderModalOpen(true);
        }}>编辑</Button>
        <Popconfirm title="确认删除?" onConfirm={() => { modelConfigApi.deleteProvider(currentTenant, record.id).then(load); }}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ];

  const configColumns = [
    { title: '模型名称', dataIndex: 'model_name', key: 'model_name', render: (v: string, r: MC) => (
      <Space>
        <span>{v}</span>
        {r.display_name && <Text type="secondary" style={{ fontSize: 12 }}>({r.display_name})</Text>}
      </Space>
    )},
    { title: 'Provider', dataIndex: 'provider_id', key: 'provider_id', render: (v: string) => {
      const p = providers.find(p => p.id === v);
      return p ? <Tag color={p.enabled !== false ? 'blue' : 'default'}>{p.name}{p.enabled === false ? ' (已禁用)' : ''}</Tag> : v;
    }},
    { title: 'Temperature', dataIndex: 'temperature', key: 'temperature', width: 100 },
    { title: 'Max Tokens', dataIndex: 'max_tokens', key: 'max_tokens', width: 100 },
    { title: '默认', dataIndex: 'is_default', key: 'is_default', width: 70, render: (v: boolean) => v ? <Tag color="gold">默认</Tag> : '-' },
    { title: '启用', dataIndex: 'enabled', key: 'enabled', width: 80, render: (v: boolean, r: MC) => (
      <Switch size="small" checked={v !== false} onChange={(checked) => onToggleConfig(r, checked)} />
    )},
    { title: '操作', key: 'action', width: 200, render: (_: unknown, record: MC) => (
      <Space>
        <Button size="small" icon={<StarOutlined />} onClick={() => onSetDefault(record.id)} disabled={record.is_default}>设为默认</Button>
        <Button size="small" icon={<EditOutlined />} onClick={() => {
          setEditingConfig(record);
          configForm.setFieldsValue(record);
          setConfigModalOpen(true);
        }}>编辑</Button>
        <Popconfirm title="确认删除?" onConfirm={() => { modelConfigApi.deleteConfig(currentTenant, record.id).then(load); }}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <Title level={3}>模型配置</Title>

      {/* 提供商 Tab */}
      <div style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <Title level={4} style={{ margin: 0 }}>模型提供商</Title>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => {
            setEditingProvider(null); providerForm.resetFields(); setProviderModalOpen(true);
          }}>新建提供商</Button>
        </div>
        <Table dataSource={providers} columns={providerColumns} rowKey="id" loading={loading} pagination={false} />
      </div>

      {/* 模型配置 Tab */}
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
          <Title level={4} style={{ margin: 0 }}>模型配置</Title>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => {
            setEditingConfig(null); configForm.resetFields(); setConfigModalOpen(true);
          }}>新建配置</Button>
        </div>
        <Table dataSource={configs} columns={configColumns} rowKey="id" loading={loading} pagination={false} />
      </div>

      {/* 提供商弹窗 */}
      <Modal
        title={editingProvider ? '编辑提供商' : '新建提供商'}
        open={providerModalOpen}
        onOk={onProviderOk}
        onCancel={() => setProviderModalOpen(false)}
      >
        <Form form={providerForm} layout="vertical">
          <Form.Item name="name" label="标识名" rules={[{ required: true }]}>
            <Input placeholder="如: openai, xiaomi, apigo" />
          </Form.Item>
          <Form.Item name="display_name" label="显示名称">
            <Input placeholder="如: OpenAI, 小米MiMo, APIGo" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'openai', label: 'OpenAI 兼容' },
              { value: 'anthropic', label: 'Anthropic' },
              { value: 'custom', label: '自定义' },
            ]} />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true }]}>
            <Input placeholder="https://api.openai.com/v1" />
          </Form.Item>
          <Form.Item name="api_key" label="API Key">
            <Input.Password placeholder="sk-..." />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      {/* 模型配置弹窗 */}
      <Modal
        title={editingConfig ? '编辑模型配置' : '新建模型配置'}
        open={configModalOpen}
        onOk={onConfigOk}
        onCancel={() => setConfigModalOpen(false)}
      >
        <Form form={configForm} layout="vertical">
          <Form.Item name="provider_id" label="提供商" rules={[{ required: true }]}>
            <Select
              options={providers
                .filter(p => p.enabled !== false)
                .map(p => ({ value: p.id, label: p.display_name ? `${p.name} (${p.display_name})` : p.name }))}
              placeholder="选择已启用的提供商"
            />
          </Form.Item>
          <Form.Item name="model_name" label="模型标识" rules={[{ required: true }]}>
            <Input placeholder="如: gpt-4o, claude-opus-4-7, mimo-v2.5-pro" />
          </Form.Item>
          <Form.Item name="display_name" label="显示名称">
            <Input placeholder="如: GPT-4o, Claude Opus 4, MiMo v2.5 Pro" />
          </Form.Item>
          <Form.Item name="temperature" label="Temperature" initialValue={0.7}>
            <InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="max_tokens" label="Max Tokens" initialValue={4096}>
            <InputNumber min={1} max={128000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="is_default" label="设为默认" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked" initialValue={true}>
            <Switch />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ModelConfigPage;
