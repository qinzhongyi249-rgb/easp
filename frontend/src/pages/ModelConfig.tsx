import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, Popconfirm, App, Switch, Dropdown, Tag, Card, Alert, Row, Col, Statistic, Steps } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SettingOutlined, MoreOutlined, StarFilled, SafetyOutlined, CheckCircleOutlined, StopOutlined, ApiOutlined, RobotOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { ModelProvider, ModelConfig, ValidateModelResponse } from '../api/modelConfig';
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
  const [validating, setValidating] = useState(false);
  const [validationResult, setValidationResult] = useState<ValidateModelResponse | null>(null);
  const isMobile = window.innerWidth < 768;
  // 供应商列表操作按钮较多，平板/窄屏也统一收纳到更多菜单，避免操作列超出可视区域。
  const useCompactActions = window.innerWidth < 1200;

  // 测试模型连接（用于模型配置）
  const testConfigConnection = async () => {
    // 获取当前供应商的 base_url 和 api_key
    const provider = providers.find(p => p.id === currentProviderId);
    if (!provider) {
      message.warning('请先选择供应商');
      return;
    }

    const modelName = configForm.getFieldValue('model_name');
    if (!modelName) {
      message.warning('请先填写模型名称');
      return;
    }
    
    setValidating(true);
    setValidationResult(null);
    
    try {
      const res = await modelConfigApi.validateModel(currentTenant, {
        base_url: provider.base_url,
        api_key: provider.api_key || '',
        model: modelName,
      });
      
      setValidationResult(res.data);
      
      if (res.data.success) {
        message.success('验证通过！');
        if (res.data.api_type) {
          message.info(`识别 API 类型：${res.data.api_type}`);
        }
        if (res.data.token_field_type) {
          message.info(`Token 字段类型：${res.data.token_field_type}`);
        }
      } else {
        message.error(`验证失败：${res.data.message}`);
      }
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '验证失败');
    } finally {
      setValidating(false);
    }
  };

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

  const defaultConfig = allConfigs.find(c => c.is_default);
  const defaultProvider = defaultConfig ? providers.find(p => p.id === defaultConfig.provider_id) : undefined;
  const enabledProviders = providers.filter(p => p.enabled !== false);
  const enabledModels = allConfigs.filter(c => c.enabled !== false);
  const disabledModels = allConfigs.filter(c => c.enabled === false);
  const fallbackCandidates = enabledModels.filter(c => !c.is_default).length;

  const getProviderModelCount = (providerId: string) => allConfigs.filter(c => c.provider_id === providerId).length;
  const getProviderEnabledModelCount = (providerId: string) => allConfigs.filter(c => c.provider_id === providerId && c.enabled !== false).length;

  // ---- 供应商列定义 ----
  const providerColumns = [
    { title: '供应商', dataIndex: 'name', key: 'name', render: (name: string, record: ModelProvider) => (
      <div>
        <Text strong>{record.display_name || name}</Text>
        <Space size={4} style={{ marginLeft: 8 }}>
          {record.enabled !== false ? <Tag color="green">供应商启用</Tag> : <Tag color="red">供应商停用</Tag>}
          <Tag color={getProviderEnabledModelCount(record.id) > 0 ? 'blue' : 'orange'}>{getProviderEnabledModelCount(record.id)}/{getProviderModelCount(record.id)} 模型可用</Tag>
        </Space>
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
    { title: '操作', key: 'action', width: useCompactActions ? 56 : 220, fixed: useCompactActions ? 'right' as const : undefined, render: (_: unknown, record: ModelProvider) => (
      useCompactActions ? (
        <Dropdown menu={{ items: [
          { key: 'addModel', label: '添加模型', icon: <PlusOutlined />, onClick: () => { setCurrentProviderId(record.id); setEditingConfig(null); configForm.resetFields(); setConfigModalOpen(true); } },
          { key: 'edit', label: '编辑供应商', icon: <EditOutlined />, onClick: () => { setEditingProvider(record); providerForm.setFieldsValue(record); setProviderModalOpen(true); } },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDeleteProvider(record.id) },
        ]}} trigger={['click']} placement="bottomRight">
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space size="small" wrap style={{ maxWidth: 220, justifyContent: 'flex-end' }}>
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
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><SettingOutlined /> 模型路由与健康</Title>
          <Text type="secondary">统一管理供应商、默认模型、备用模型、连接验证和调用风险。AI 助手只应使用启用且验证可用的模型。</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditingProvider(null); providerForm.resetFields(); setProviderModalOpen(true); }}>新建供应商</Button>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
        <Alert
          type={defaultConfig && defaultProvider?.enabled !== false && defaultConfig.enabled !== false ? 'success' : 'warning'}
          showIcon
          message={defaultConfig ? `当前默认模型：${defaultProvider?.display_name || defaultProvider?.name || '-'} / ${defaultConfig.display_name || defaultConfig.model_name}` : '尚未设置默认模型'}
          description="建议先测试连接再设为默认。默认模型异常时，应保留至少一个启用的备用模型用于 fallback；不要静默 fallback 到错误配置。"
        />
        <Steps
          size="small"
          current={defaultConfig ? 3 : Math.min(2, enabledModels.length)}
          items={[
            { title: '配置供应商', description: 'Base URL / API Key' },
            { title: '添加模型', description: '名称、温度、Token 上限' },
            { title: '测试连接', description: '识别 API/流式/工具/Token' },
            { title: '设为默认', description: 'AI 助手生产调用入口' },
          ]}
        />
        <Row gutter={[16, 16]}>
          <Col xs={12} lg={6}><Card><Statistic title="供应商" value={providers.length} prefix={<ApiOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="启用供应商" value={enabledProviders.length} prefix={<CheckCircleOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="启用模型" value={enabledModels.length} prefix={<RobotOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="备用模型" value={fallbackCandidates} prefix={<ThunderboltOutlined />} /></Card></Col>
        </Row>
        <Card size="small" title="模型路由摘要" extra={<Text type="secondary">基于当前配置</Text>}>
          <Space wrap>
            <Tag color={defaultConfig ? 'gold' : 'orange'} icon={defaultConfig ? <StarFilled /> : undefined}>默认：{defaultConfig ? `${defaultProvider?.display_name || defaultProvider?.name || '-'} / ${defaultConfig.model_name}` : '未设置'}</Tag>
            <Tag color={fallbackCandidates > 0 ? 'green' : 'orange'}>Fallback候选：{fallbackCandidates}</Tag>
            <Tag color={disabledModels.length ? 'red' : 'green'} icon={disabledModels.length ? <StopOutlined /> : <CheckCircleOutlined />}>停用模型：{disabledModels.length}</Tag>
          </Space>
        </Card>
      </Space>

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
        title={editingProvider ? '编辑模型供应商' : '新建模型供应商'}
        open={providerModalOpen}
        onOk={onProviderOk}
        onCancel={() => setProviderModalOpen(false)}
        width={isMobile ? '90%' : 600}
        footer={[
          <Button key="cancel" onClick={() => setProviderModalOpen(false)}>取消</Button>,
          <Button key="submit" type="primary" onClick={onProviderOk}>确定</Button>,
        ]}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="供应商只保存连接信息，具体模型在供应商下单独配置。"
          description="API Key 属于敏感信息；保存后页面不应依赖前端明文做业务决策，模型调用由后端统一路由。"
        />
        <Form form={providerForm} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input placeholder="如 openai, xiaomi" /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input placeholder="如 小米，OpenAI" /></Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}><Input placeholder="openai / anthropic / custom" /></Form.Item>
          <Form.Item name="base_url" label="基础 URL" rules={[{ required: true }]}><Input placeholder="https://api.openai.com/v1" /></Form.Item>
          <Form.Item name="api_key" label="API Key" rules={[{ required: true }]}><Input.Password /></Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>

{/* 模型配置弹窗 */}
      <Modal
        title={editingConfig ? '编辑模型路由' : '添加模型路由'}
        open={configModalOpen}
        onOk={onConfigOk}
        onCancel={() => setConfigModalOpen(false)}
        width={isMobile ? '90%' : 600}
        footer={[
          <Button key="test" icon={<SafetyOutlined />} onClick={testConfigConnection} loading={validating}>
            测试连接
          </Button>,
          <Button key="cancel" onClick={() => setConfigModalOpen(false)}>取消</Button>,
          <Button key="submit" type="primary" onClick={onConfigOk}>确定</Button>,
        ]}
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="建议先测试连接，再保存或设为默认。"
          description="测试会识别 API 类型、响应格式、流式支持、工具调用支持和 Token 字段类型。默认模型失败时应明确报错或走后端受控 fallback，禁止静默走错模型。"
        />
        <Form form={configForm} layout="vertical" size={isMobile ? 'middle' : 'large'}
          initialValues={{ temperature: 0.7, max_tokens: 4096, is_default: false, enabled: true }}
        >
          <Form.Item name="model_name" label="模型名称" rules={[{ required: true, message: '请输入模型名称' }]}><Input placeholder="如 gpt-4o, mimo-v2.5-pro" /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input placeholder="如 GPT-4o, MiMo Pro" /></Form.Item>
          <Form.Item name="temperature" label="温度"><InputNumber min={0} max={2} step={0.1} style={{ width: '100%' }} placeholder="0.0 - 2.0" /></Form.Item>
          <Form.Item name="max_tokens" label="最大 Token 数"><InputNumber min={1} max={1000000} step={256} style={{ width: '100%' }} placeholder="如 4096" /></Form.Item>
          <Form.Item name="is_default" label="设为默认" valuePropName="checked"><Switch /></Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
        
        {validationResult && (
          <Alert
            message={validationResult.success ? "验证成功" : "验证失败"}
            description={
              <div style={{ fontSize: 13 }}>
                <div>{validationResult.message}</div>
                {validationResult.api_type && <div>• API 类型：{validationResult.api_type}</div>}
                {validationResult.response_type && <div>• 响应格式：{validationResult.response_type}</div>}
                {validationResult.token_field_type && <div>• Token 字段类型：{validationResult.token_field_type}</div>}
                {validationResult.supports_tools !== undefined && <div>• 支持工具调用：{validationResult.supports_tools ? '是' : '否'}</div>}
                {validationResult.supports_stream !== undefined && <div>• 支持流式：{validationResult.supports_stream ? '是' : '否'}</div>}
              </div>
            }
            type={validationResult.success ? 'success' : 'error'}
            showIcon
            style={{ marginTop: 16 }}
          />
        )}
      </Modal>
    </div>
  );
};

export default ModelConfigPage;
