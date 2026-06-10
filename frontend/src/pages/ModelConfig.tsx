import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, Popconfirm, App, Switch, Dropdown, Tag, Card } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SettingOutlined, MoreOutlined, StarFilled } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { ModelProvider, ModelConfig } from '../api/modelConfig';
import { modelConfigApi } from '../api/modelConfig';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

const ModelConfigPage: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [providers, setProviders] = useState<ModelProvider[]>([]);
  const [allConfigs, setAllConfigs] = useState<ModelConfig[]>([]);
  const [loading, setLoading] = useState(false);
  const [providerModalOpen, setProviderModalOpen] = useState(false);
  const [configModalOpen, setConfigModalOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<ModelProvider | null>(null);
  const [editingConfig, setEditingConfig] = useState<ModelConfig | null>(null);
  const [currentProviderId, setCurrentProviderId] = useState<string>('');
  const [providerForm] = Form.useForm();
  const [configForm] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const [provRes, cfgRes] = await Promise.all([
        modelConfigApi.listProviders(currentTenant),
        modelConfigApi.listConfigs(currentTenant),
      ]);
      setProviders(provRes.data || []);
      setAllConfigs(cfgRes.data || []);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  // ---- Provider 操作 ----
  const onProviderOk = async () => {
    const values = await providerForm.validateFields();
    try {
      if (editingProvider) { await modelConfigApi.updateProvider(currentTenant, editingProvider.id, values); message.success('更新成功'); }
      else { await modelConfigApi.createProvider(currentTenant, values); message.success('创建成功'); }
      setProviderModalOpen(false); providerForm.resetFields(); setEditingProvider(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDeleteProvider = async (id: string) => {
    try { await modelConfigApi.deleteProvider(currentTenant, id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const onToggleProvider = async (id: string, enabled: boolean) => {
    try { await modelConfigApi.updateProvider(currentTenant, id, { enabled }); message.success(enabled ? '已启用' : '已禁用'); load(); }
    catch { message.error('操作失败'); }
  };

  // ---- Config 操作 ----
  const onConfigOk = async () => {
    const values = await configForm.validateFields();
    try {
      // 类型转换：InputNumber 已返回 number，但兜底处理
      const payload: Record<string, unknown> = {
        ...values,
        provider_id: currentProviderId,
        temperature: values.temperature != null ? Number(values.temperature) : undefined,
        max_tokens: values.max_tokens != null ? Math.round(Number(values.max_tokens)) : undefined,
      };
      // 移除 undefined 字段，避免覆盖后端默认值
      Object.keys(payload).forEach(k => { if (payload[k] === undefined) delete payload[k]; });
      if (editingConfig) { await modelConfigApi.updateConfig(currentTenant, editingConfig.id, payload); message.success('更新成功'); }
      else { await modelConfigApi.createConfig(currentTenant, payload); message.success('创建成功'); }
      setConfigModalOpen(false); configForm.resetFields(); setEditingConfig(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDeleteConfig = async (id: string) => {
    try { await modelConfigApi.deleteConfig(currentTenant, id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const onSetDefault = async (id: string) => {
    try { await modelConfigApi.setDefault(currentTenant, id); message.success('已设为默认'); load(); }
    catch { message.error('操作失败'); }
  };

  const onToggleConfig = async (id: string, enabled: boolean) => {
    try { await modelConfigApi.updateConfig(currentTenant, id, { enabled }); message.success(enabled ? '已启用' : '已禁用'); load(); }
    catch { message.error('操作失败'); }
  };

  // ---- 供应商列定义 ----
  const providerColumns = [
    { title: '供应商', dataIndex: 'name', key: 'name', render: (name: string, record: ModelProvider) => (
      <div>
        <Text strong>{record.display_name || name}</Text>
        {isMobile && <div><Text type="secondary" style={{ fontSize: 12 }}>{record.type} · {record.base_url}</Text></div>}
      </div>
    )},
    ...(!isMobile ? [
      { title: '类型', dataIndex: 'type', key: 'type', width: 100 },
      { title: '基础URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
    ] : []),
    { title: '状态', key: 'status', width: 70, render: (_: unknown, record: ModelProvider) => (
      <Switch size="small" checked={record.enabled} onChange={(checked) => onToggleProvider(record.id, checked)} />
    )},
    { title: '操作', key: 'action', width: isMobile ? 50 : 150, render: (_: unknown, record: ModelProvider) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'addModel', label: '添加模型', icon: <PlusOutlined />, onClick: () => { setCurrentProviderId(record.id); setEditingConfig(null); configForm.resetFields(); setConfigModalOpen(true); } },
          { key: 'edit', label: '编辑供应商', icon: <EditOutlined />, onClick: () => { setEditingProvider(record); providerForm.setFieldsValue(record); setProviderModalOpen(true); } },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDeleteProvider(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<PlusOutlined />} onClick={() => { setCurrentProviderId(record.id); setEditingConfig(null); configForm.resetFields(); setConfigModalOpen(true); }}>添加模型</Button>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setEditingProvider(record); providerForm.setFieldsValue(record); setProviderModalOpen(true); }}>编辑</Button>
          <Popconfirm title="删除供应商将同时删除其下所有模型，确认?" onConfirm={() => onDeleteProvider(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />} />
          </Popconfirm>
        </Space>
      )
    )},
  ];

  // ---- 展开行：显示该供应商下的模型配置 ----
  const expandedRowRender = (provider: ModelProvider) => {
    const configs = allConfigs.filter(c => c.provider_id === provider.id);
    if (configs.length === 0) {
      return (
        <div style={{ padding: '8px 0' }}>
          <Text type="secondary">暂无模型配置，点击上方「添加模型」创建</Text>
        </div>
      );
    }
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {configs.map(cfg => (
          <Card key={cfg.id} size="small" style={{ background: '#fafafa' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}>
              <div>
                <Space>
                  <Text strong>{cfg.display_name || cfg.model_name}</Text>
                  {cfg.is_default && <Tag color="gold" icon={<StarFilled />}>默认</Tag>}
                  {!cfg.enabled && <Tag color="red">已禁用</Tag>}
                </Space>
                <div>
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {cfg.model_name}
                    {cfg.temperature !== undefined && ` · 温度: ${cfg.temperature}`}
                    {cfg.max_tokens !== undefined && ` · 最大Token: ${cfg.max_tokens}`}
                  </Text>
                </div>
              </div>
              <Space size="small" wrap>
                <Switch size="small" checked={cfg.enabled !== false} onChange={(checked) => onToggleConfig(cfg.id, checked)} />
                {!cfg.is_default && <Button size="small" onClick={() => onSetDefault(cfg.id)}>设为默认</Button>}
                <Button size="small" icon={<EditOutlined />} onClick={() => { setEditingConfig(cfg); setCurrentProviderId(provider.id); configForm.setFieldsValue(cfg); setConfigModalOpen(true); }} />
                <Popconfirm title="确认删除?" onConfirm={() => onDeleteConfig(cfg.id)}>
                  <Button size="small" danger icon={<DeleteOutlined />} />
                </Popconfirm>
              </Space>
            </div>
          </Card>
        ))}
      </div>
    );
  };

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><SettingOutlined /> 模型配置</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingProvider(null); providerForm.resetFields(); setProviderModalOpen(true); }}>新建供应商</Button>
      </div>
      <Table
        dataSource={providers}
        columns={providerColumns}
        rowKey="id"
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        expandable={{
          expandedRowRender,
          rowExpandable: () => true,
        }}
        pagination={false}
      />

      {/* 供应商弹窗 */}
      <Modal
        title={editingProvider ? '编辑供应商' : '新建供应商'}
        open={providerModalOpen}
        onOk={onProviderOk}
        onCancel={() => setProviderModalOpen(false)}
        width={isMobile ? '90%' : 500}
      >
        <Form form={providerForm} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input placeholder="如 openai, xiaomi" /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input placeholder="如 小米, OpenAI" /></Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}><Input placeholder="openai / anthropic / custom" /></Form.Item>
          <Form.Item name="base_url" label="基础URL" rules={[{ required: true }]}><Input placeholder="https://api.openai.com/v1" /></Form.Item>
          <Form.Item name="api_key" label="API Key" rules={[{ required: true }]}><Input.Password /></Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>

      {/* 模型配置弹窗 */}
      <Modal
        title={editingConfig ? '编辑模型' : '添加模型'}
        open={configModalOpen}
        onOk={onConfigOk}
        onCancel={() => setConfigModalOpen(false)}
        width={isMobile ? '90%' : 500}
      >
        <Form form={configForm} layout="vertical" size={isMobile ? 'middle' : 'large'}
          initialValues={{ temperature: 0.7, max_tokens: 4096, is_default: false, enabled: true }}
        >
          <Form.Item name="model_name" label="模型名称" rules={[{ required: true, message: '请输入模型名称' }]}><Input placeholder="如 gpt-4o, mimo-v2.5-pro" /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input placeholder="如 GPT-4o, MiMo Pro" /></Form.Item>
          <Form.Item name="temperature" label="温度"><InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} placeholder="0.0 - 2.0" /></Form.Item>
          <Form.Item name="max_tokens" label="最大Token数"><InputNumber min={1} max={1000000} step={256} style={{ width: '100%' }} placeholder="如 4096" /></Form.Item>
          <Form.Item name="is_default" label="设为默认" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ModelConfigPage;
