import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, App, Tag, Dropdown, Select, Divider, Switch, Alert, Card, Statistic, Steps, Empty, Drawer, Descriptions } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined, MoreOutlined, MinusCircleOutlined, PlusCircleOutlined, CloudServerOutlined, CodeOutlined, SafetyCertificateOutlined, ToolOutlined, LockOutlined, EyeOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Connector } from '../api/connector';
import { connectorApi } from '../api/connector';

const { Title, Text, Paragraph } = Typography;
interface LayoutContext { currentTenant: string; }

const fmtTime = (value?: string) => value ? new Date(value).toLocaleString() : '-';
const prettyJson = (value?: string) => {
  if (!value) return '无';
  try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
};

const Connectors: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Connector | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailConnector, setDetailConnector] = useState<Connector | null>(null);
  const [form] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  // 认证/凭据类型联动
  const authType = Form.useWatch('auth_type', form);
  const credentialMode = Form.useWatch('credential_mode', form);

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { const res = await connectorApi.list(currentTenant); setConnectors(res.data || []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const isLockedBuiltinConnector = (connector?: Connector | null) => Boolean(connector?.is_builtin || connector?.locked || connector?.type === 'builtin');
  const openDetail = (connector: Connector) => { setDetailConnector(connector); setDetailOpen(true); };
  const connectorState = (connector: Connector) => {
    if (isLockedBuiltinConnector(connector)) return { label: '内置锁定', type: 'info' as const, desc: '系统内置接入源不可编辑或删除，后端 API 会强制拦截。' };
    if (connector.status !== 'active') return { label: '未启用/异常', type: 'warning' as const, desc: '接入源不是 active，相关工具可能不可用于生产调用。' };
    if (connector.type === 'mcp' && !connector.mcp_server_url) return { label: '缺 MCP 地址', type: 'warning' as const, desc: 'MCP 接入源缺少 Server 地址，无法自动发现工具。' };
    if ((connector.type === 'rest' || connector.type === 'openapi') && !connector.base_url) return { label: '缺基础 URL', type: 'warning' as const, desc: 'REST/OpenAPI 接入源缺少基础 URL，导入工具后可能无法调用。' };
    return { label: '可治理', type: 'success' as const, desc: '接入源状态正常，可继续导入工具并进入角色授权。' };
  };

  // 打开编辑弹窗时，解析 headers JSON 为 KV 数组
  const openModal = useCallback((record?: Connector) => {
    if (record && isLockedBuiltinConnector(record)) {
      message.warning('内置锁定接入源不可编辑');
      return;
    }
    if (record) {
      setEditing(record);
      // 解析 headers JSON → KV 数组
      let headersKV: { key: string; value: string }[] = [];
      if (record.headers) {
        try {
          const obj = JSON.parse(record.headers);
          headersKV = Object.entries(obj).map(([k, v]) => ({ key: k, value: String(v) }));
        } catch { /* ignore */ }
      }
      if (headersKV.length === 0) headersKV = [{ key: '', value: '' }];

      // 解析 auth_config JSON
      let authConfigStr = record.auth_config || '';
      if (authConfigStr) {
        try { authConfigStr = JSON.stringify(JSON.parse(authConfigStr), null, 2); } catch { /* keep raw */ }
      }
      const credentialMode = record.credential_mode || (record.auth_type ? 'static' : 'none');

      form.setFieldsValue({
        ...record,
        credential_mode: credentialMode,
        auth_type: record.auth_type || 'none',
        user_token_header: record.user_token_header || 'Authorization',
        user_token_prefix: record.user_token_prefix ?? 'Bearer',
        user_token_required_sso: record.user_token_required_sso ?? true,
        headers_kv: headersKV,
        auth_config_text: authConfigStr,
      });
    } else {
      setEditing(null);
      form.resetFields();
      form.setFieldsValue({
        type: 'mcp',
        auth_type: 'none',
        credential_mode: 'static',
        user_token_header: 'Authorization',
        user_token_prefix: 'Bearer',
        user_token_required_sso: true,
        transport_type: 'streamable_http',
        headers_kv: [{ key: '', value: '' }],
      });
    }
    setModalOpen(true);
  }, [form, message]);

  const onOk = async () => {
    const values = await form.validateFields();

    // 转换 headers KV 数组 → JSON
    const headersObj: Record<string, string> = {};
    (values.headers_kv || []).forEach((item: { key: string; value: string }) => {
      if (item.key && item.key.trim()) {
        headersObj[item.key.trim()] = item.value || '';
      }
    });
    const headersJSON = Object.keys(headersObj).length > 0 ? JSON.stringify(headersObj) : null;

    // 解析 auth_config
    let authConfig = values.auth_config_text || null;
    if (values.credential_mode === 'static' && authConfig && authConfig.trim()) {
      try { JSON.parse(authConfig); } catch { message.error('认证配置必须是合法JSON'); return; }
    }

    const payload: Partial<Connector> = {
      name: values.name,
      type: values.type,
      base_url: values.base_url,
      transport_type: values.transport_type || undefined,
      mcp_server_url: values.mcp_server_url || undefined,
      headers: headersJSON || undefined,
      auth_type: values.credential_mode === 'static' && values.auth_type !== 'none' ? values.auth_type : undefined,
      auth_config: values.credential_mode === 'static' && authConfig ? authConfig : undefined,
      credential_mode: values.credential_mode || 'static',
      user_token_header: values.user_token_header || undefined,
      user_token_prefix: values.user_token_prefix ?? undefined,
      user_token_required_sso: values.user_token_required_sso ?? true,
      status: 'active',
    };

    try {
      if (editing) { await connectorApi.update(currentTenant, editing.id, payload); message.success('更新成功'); }
      else { await connectorApi.create(currentTenant, payload); message.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDelete = async (connector: Connector) => {
    if (isLockedBuiltinConnector(connector)) {
      message.warning('内置锁定接入源不可删除');
      return;
    }
    try { await connectorApi.delete(currentTenant, connector.id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const credentialModeLabel = (mode?: string) => {
    if (mode === 'user_token') return <Tag color="purple">用户 Token 透传</Tag>;
    if (mode === 'none') return <Tag>无认证</Tag>;
    return <Tag color="orange">静态凭据</Tag>;
  };

  const connectorSummary = {
    total: connectors.length,
    mcp: connectors.filter(item => item.type === 'mcp').length,
    openapi: connectors.filter(item => item.type === 'openapi' || item.type === 'rest').length,
    userToken: connectors.filter(item => item.credential_mode === 'user_token').length,
    tools: connectors.reduce((sum, item) => sum + (item.tools_count || 0), 0),
  };

  const openConnectorWizard = (type: Connector['type'], credentialMode: Connector['credential_mode'] = 'static') => {
    openModal();
    setTimeout(() => {
      form.setFieldsValue({
        type,
        credential_mode: credentialMode,
        transport_type: type === 'mcp' ? 'streamable_http' : undefined,
        auth_type: credentialMode === 'static' ? 'none' : undefined,
        user_token_header: 'Authorization',
        user_token_prefix: 'Bearer',
        user_token_required_sso: true,
      });
    }, 0);
  };

  const renderConnectorCard = (connector: Connector) => (
    <Card key={connector.id} size="small" style={{ height: '100%' }}>
      <Space direction="vertical" size="small" style={{ width: '100%' }}>
        <Space wrap style={{ justifyContent: 'space-between', width: '100%' }}>
          <Text strong>{connector.name}</Text>
          <Tag color={connector.status === 'active' ? 'green' : 'red'}>{connector.status}</Tag>
        </Space>
        <Space wrap>
          <Tag color={connector.type === 'mcp' ? 'blue' : connector.type === 'builtin' ? 'purple' : 'cyan'}>{connector.type}</Tag>
          {isLockedBuiltinConnector(connector) && <Tag color="purple" icon={<LockOutlined />}>内置锁定</Tag>}
          {connector.transport_type && <Tag color={connector.transport_type === 'sse' ? 'geekblue' : 'green'}>{connector.transport_type}</Tag>}
          {credentialModeLabel(connector.credential_mode || (connector.auth_type ? 'static' : 'none'))}
        </Space>
        <Paragraph ellipsis={{ rows: 1 }} style={{ marginBottom: 0 }} copyable={!!connector.base_url}>{connector.base_url || connector.mcp_server_url || '-'}</Paragraph>
        <Text type="secondary">已导入工具：{connector.tools_count || 0}</Text>
        <Space wrap>
          <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(connector)}>详情</Button>
          <Button size="small" icon={<EditOutlined />} disabled={isLockedBuiltinConnector(connector)} onClick={() => openModal(connector)}>配置</Button>
          <Button size="small" icon={<ToolOutlined />} href="/mcp-tools">工具管理</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(connector)} disabled={isLockedBuiltinConnector(connector)}>
            <Button size="small" danger icon={<DeleteOutlined />} disabled={isLockedBuiltinConnector(connector)}>删除</Button>
          </Popconfirm>
        </Space>
      </Space>
    </Card>
  );

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, record: Connector) => <Space wrap><Text>{v}</Text>{isLockedBuiltinConnector(record) && <Tag color="purple" icon={<LockOutlined />}>内置锁定</Tag>}</Space> },
    ...(!isMobile ? [
      { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (v: string) => <Tag>{v}</Tag> },
      { title: '传输', dataIndex: 'transport_type', key: 'transport_type', width: 100, render: (v: string) => v ? <Tag color={v === 'sse' ? 'blue' : 'green'}>{v}</Tag> : '-' },
      { title: '基础URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
      { title: 'MCP地址', dataIndex: 'mcp_server_url', key: 'mcp_server_url', ellipsis: true, render: (v: string) => v || '-' },
      { title: '认证', dataIndex: 'auth_type', key: 'auth_type', width: 80, render: (v: string) => v ? <Tag color="orange">{v}</Tag> : '-' },
    ] : []),
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'red'}>{v}</Tag> },
    { title: '操作', key: 'action', width: isMobile ? 60 : 210, render: (_: unknown, record: Connector) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'detail', label: '详情', icon: <EyeOutlined />, onClick: () => openDetail(record) },
          { key: 'edit', label: isLockedBuiltinConnector(record) ? '内置不可编辑' : '编辑', icon: <EditOutlined />, disabled: isLockedBuiltinConnector(record), onClick: () => openModal(record) },
          { key: 'delete', label: isLockedBuiltinConnector(record) ? '内置不可删除' : '删除', icon: <DeleteOutlined />, danger: true, disabled: isLockedBuiltinConnector(record), onClick: () => onDelete(record) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>详情</Button>
          <Button size="small" icon={<EditOutlined />} disabled={isLockedBuiltinConnector(record)} onClick={() => openModal(record)}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(record)} disabled={isLockedBuiltinConnector(record)}>
            <Button size="small" danger icon={<DeleteOutlined />} disabled={isLockedBuiltinConnector(record)}>删除</Button>
          </Popconfirm>
        </Space>
      )
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><ApiOutlined /> 业务工具接入</Title>
          <Text type="secondary">把业务系统 API、OpenAPI 文档或 MCP Server 接入 EASP，再统一做权限、审计和助手调用。</Text>
        </div>
        <Space wrap>
          <Button icon={<ToolOutlined />} href="/mcp-tools">查看工具</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openConnectorWizard('mcp')}>新建接入</Button>
        </Space>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="业务工具接入是增强能力：把内部 REST/OpenAPI/MCP 能力变成可治理工具。"
          description="权限主体仍然是 EASP 内部用户；外部业务系统嵌入助手后，工具/Skill/MCP 的获取和执行链路都会按 EASP 角色与权限判断。内置/锁定接入源不可编辑或删除，后端 API 也会强制拦截。"
        />

        <Card>
          <Steps
            size="small"
            direction={isMobile ? 'vertical' : 'horizontal'}
            items={[
              { title: '选择接入方式', description: 'MCP / OpenAPI / REST' },
              { title: '配置凭据模式', description: '静态 / 用户Token / 无认证' },
              { title: '导入工具', description: '发现或创建 MCP Tool' },
              { title: '授权与审计', description: '角色授权后给助手使用' },
            ]}
          />
        </Card>

        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(4, minmax(0, 1fr))', gap: 12 }}>
          <Card><Statistic title="接入源" value={connectorSummary.total} prefix={<CloudServerOutlined />} /></Card>
          <Card><Statistic title="MCP Server" value={connectorSummary.mcp} prefix={<ApiOutlined />} /></Card>
          <Card><Statistic title="REST/OpenAPI" value={connectorSummary.openapi} prefix={<CodeOutlined />} /></Card>
          <Card><Statistic title="已导入工具" value={connectorSummary.tools} prefix={<ToolOutlined />} /></Card>
        </div>

        <Card title="选择接入方式" extra={<Text type="secondary">先建连接器，再去 MCP 工具页导入/授权</Text>}>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
            <Card size="small" hoverable onClick={() => openConnectorWizard('mcp')}>
              <Space direction="vertical">
                <Space><ApiOutlined /><Text strong>MCP Server 接入</Text><Tag color="green">推荐</Tag></Space>
                <Text type="secondary">接入 StreamableHTTP 或 SSE MCP Server，自动发现工具。</Text>
                <Button size="small" type="primary">接入 MCP</Button>
              </Space>
            </Card>
            <Card size="small" hoverable onClick={() => openConnectorWizard('openapi')}>
              <Space direction="vertical">
                <Space><CodeOutlined /><Text strong>OpenAPI 导入</Text><Tag color="blue">批量</Tag></Space>
                <Text type="secondary">先创建 OpenAPI 连接器，再在 MCP 工具页导入接口为工具。</Text>
                <Button size="small">接入 OpenAPI</Button>
              </Space>
            </Card>
            <Card size="small" hoverable onClick={() => openConnectorWizard('rest', 'user_token')}>
              <Space direction="vertical">
                <Space><SafetyCertificateOutlined /><Text strong>业务用户 Token 透传</Text><Tag color="purple">SSO</Tag></Space>
                <Text type="secondary">工具调用时透传当前 SSO 用户业务 Token，无 Token 明确失败。</Text>
                <Button size="small">配置透传</Button>
              </Space>
            </Card>
          </div>
        </Card>

        <Card title="接入源" extra={<Button icon={<PlusOutlined />} onClick={() => openConnectorWizard('mcp')}>新建接入源</Button>}>
          {connectors.length === 0 ? <Empty description="暂无业务工具接入源，先选择上方接入方式创建" /> : (
            <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
              {connectors.map(renderConnectorCard)}
            </div>
          )}
        </Card>

        <Card title="接入源明细">
          <Table
            dataSource={connectors}
            columns={columns}
            rowKey="id"
            loading={loading}
            size={isMobile ? 'small' : 'middle'}
            scroll={isMobile ? { x: 400 } : undefined}
          />
        </Card>
      </Space>
      <Drawer
        title={`接入源治理详情 — ${detailConnector?.name || ''}`}
        open={detailOpen}
        onClose={() => setDetailOpen(false)}
        width={isMobile ? '100%' : 720}
      >
        {detailConnector && (() => {
          const state = connectorState(detailConnector);
          const mode = detailConnector.credential_mode || (detailConnector.auth_type ? 'static' : 'none');
          return (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Alert type={state.type} showIcon message={state.label} description={state.desc} />
              <Descriptions bordered size="small" column={isMobile ? 1 : 2}>
                <Descriptions.Item label="接入源ID">{detailConnector.id}</Descriptions.Item>
                <Descriptions.Item label="来源">{isLockedBuiltinConnector(detailConnector) ? <Tag color="purple" icon={<LockOutlined />}>系统内置</Tag> : <Tag>租户自定义</Tag>}</Descriptions.Item>
                <Descriptions.Item label="名称">{detailConnector.name}</Descriptions.Item>
                <Descriptions.Item label="类型"><Tag color={detailConnector.type === 'builtin' ? 'purple' : detailConnector.type === 'mcp' ? 'blue' : 'cyan'}>{detailConnector.type}</Tag></Descriptions.Item>
                <Descriptions.Item label="状态"><Tag color={detailConnector.status === 'active' ? 'green' : 'red'}>{detailConnector.status}</Tag></Descriptions.Item>
                <Descriptions.Item label="锁定状态">{isLockedBuiltinConnector(detailConnector) ? <Tag color="purple">内置锁定</Tag> : <Tag>可编辑</Tag>}</Descriptions.Item>
                <Descriptions.Item label="传输方式">{detailConnector.transport_type ? <Tag color={detailConnector.transport_type === 'sse' ? 'blue' : 'green'}>{detailConnector.transport_type}</Tag> : '-'}</Descriptions.Item>
                <Descriptions.Item label="凭据模式">{credentialModeLabel(mode)}</Descriptions.Item>
                <Descriptions.Item label="认证类型">{detailConnector.auth_type || '-'}</Descriptions.Item>
                <Descriptions.Item label="已导入工具">{detailConnector.tools_count || 0}</Descriptions.Item>
                <Descriptions.Item label="创建时间">{fmtTime(detailConnector.created_at)}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{fmtTime(detailConnector.updated_at)}</Descriptions.Item>
                <Descriptions.Item label="基础URL" span={isMobile ? 1 : 2}>{detailConnector.base_url || '-'}</Descriptions.Item>
                <Descriptions.Item label="MCP Server" span={isMobile ? 1 : 2}>{detailConnector.mcp_server_url || '-'}</Descriptions.Item>
                {mode === 'user_token' && <Descriptions.Item label="用户 Token 透传" span={isMobile ? 1 : 2}>{detailConnector.user_token_header || 'Authorization'} {detailConnector.user_token_prefix || ''}；{detailConnector.user_token_required_sso === false ? '不强制 SSO Token' : '要求 SSO Token'}</Descriptions.Item>}
              </Descriptions>
              <Card size="small" title="自定义请求头">
                <pre style={{ maxHeight: 180, overflow: 'auto', background: '#f5f5f5', padding: 12, borderRadius: 6 }}>{prettyJson(detailConnector.headers)}</pre>
              </Card>
              <Card size="small" title="治理建议">
                <Space direction="vertical">
                  <Text>1. 接入源只负责连接业务系统，真正暴露给助手的是 MCP Tool。</Text>
                  <Text>2. 静态凭据只应保存在服务端；用户 Token 透传无 SSO Token 时应明确失败。</Text>
                  <Text>3. 内置锁定接入源由系统治理链路保留，不可编辑或删除。</Text>
                  <Text>4. 导入工具后，还需要发布、启用并在角色授权中分配，助手才可调用。</Text>
                </Space>
              </Card>
            </Space>
          );
        })()}
      </Drawer>
      <Modal
        title={editing ? '编辑业务工具接入源' : '新建业务工具接入源'}
        open={modalOpen}
        onOk={onOk}
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '95%' : 760}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ type: 'mcp', auth_type: 'none', credential_mode: 'static', user_token_header: 'Authorization', user_token_prefix: 'Bearer', user_token_required_sso: true, transport_type: 'streamable_http', headers_kv: [{ key: '', value: '' }] }}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="接入源负责连接业务系统，真正暴露给助手的是 MCP 工具。"
            description="创建后可到“工具管理”导入 OpenAPI/REST/MCP 工具，再通过角色权限决定谁能使用。"
          />
          {/* 基本信息 */}
          <Form.Item name="name" label="连接器名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如: GitHub MCP / 企业数据服务" />
          </Form.Item>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="type" label="连接器类型" rules={[{ required: true }]} style={{ flex: 1 }}>
              <Select options={[
                { value: 'mcp', label: 'MCP 服务' },
                { value: 'openapi', label: 'OpenAPI' },
                { value: 'rest', label: 'REST API' },
                { value: 'custom', label: '自定义' },
              ]} />
            </Form.Item>
            <Form.Item name="transport_type" label="MCP传输方式" style={{ flex: 1 }}>
              <Select options={[
                { value: 'streamable_http', label: 'StreamableHTTP (推荐)' },
                { value: 'sse', label: 'SSE (旧版)' },
              ]} />
            </Form.Item>
          </div>

          <Form.Item name="base_url" label="基础URL" rules={[{ required: true, message: '请输入URL' }]}>
            <Input placeholder="https://api.example.com" />
          </Form.Item>

          <Form.Item name="mcp_server_url" label="MCP Server地址" tooltip="填写MCP Server的端点地址，用于自动发现和导入MCP工具">
            <Input placeholder="http://localhost:3000/mcp 或 /sse (可选)" />
          </Form.Item>

          <Divider style={{ margin: '12px 0 16px' }}>自定义HTTP头</Divider>

          {/* 自定义头 KV 编辑器 */}
          <Form.List name="headers_kv">
            {(fields, { add, remove }) => (
              <>
                {fields.map(({ key, name, ...restField }) => (
                  <div key={key} style={{ display: 'flex', gap: 8, marginBottom: 8, alignItems: 'center' }}>
                    <Form.Item {...restField} name={[name, 'key']} style={{ flex: 1, marginBottom: 0 }}>
                      <Input placeholder="Header Name (如 X-Custom-Token)" />
                    </Form.Item>
                    <Form.Item {...restField} name={[name, 'value']} style={{ flex: 1, marginBottom: 0 }}>
                      <Input placeholder="Header Value" />
                    </Form.Item>
                    {fields.length > 1 && (
                      <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#999', cursor: 'pointer' }} />
                    )}
                  </div>
                ))}
                <Button type="dashed" onClick={() => add()} icon={<PlusCircleOutlined />} size="small" style={{ width: '100%' }}>
                  添加自定义头
                </Button>
              </>
            )}
          </Form.List>

          <Divider style={{ margin: '16px 0 16px' }}>认证配置</Divider>

          <Form.Item name="credential_mode" label="凭据模式" tooltip="静态凭据使用连接器固定密钥；用户Token透传会使用当前SSO登录用户的业务Token；无认证不发送认证信息。">
            <Select options={[
              { value: 'static', label: '静态凭据' },
              { value: 'user_token', label: '透传当前 SSO 用户 Token' },
              { value: 'none', label: '无认证' },
            ]} />
          </Form.Item>

          {credentialMode === 'user_token' && (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 12 }}
                message="仅 SSO 登录用户可用"
                description="调用 MCP/REST 工具时，后端会读取当前用户最近一次 SSO 登录保存的业务 Token 并动态注入请求头；无 Token 会明确失败。"
              />
              <div style={{ display: 'flex', gap: 16 }}>
                <Form.Item name="user_token_header" label="Token Header" rules={[{ required: true, message: '请输入Header名称' }]} style={{ flex: 1 }}>
                  <Input placeholder="Authorization" />
                </Form.Item>
                <Form.Item name="user_token_prefix" label="Token 前缀" tooltip="为空则直接写入Token；常用 Bearer / Token" style={{ flex: 1 }}>
                  <Input placeholder="Bearer" />
                </Form.Item>
              </div>
              <Form.Item name="user_token_required_sso" label="要求 SSO Token" valuePropName="checked" tooltip="开启后，无业务 SSO Token 时明确失败，不降级到静态凭据。">
                <Switch checkedChildren="要求" unCheckedChildren="不要求" />
              </Form.Item>
            </>
          )}

          {credentialMode === 'static' && (
            <>
              <Form.Item name="auth_type" label="认证类型">
                <Select options={[
                  { value: 'none', label: '无认证' },
                  { value: 'bearer', label: 'Bearer Token' },
                  { value: 'api_key', label: 'API Key' },
                  { value: 'basic', label: 'Basic Auth' },
                ]} />
              </Form.Item>

              {authType && authType !== 'none' && (
                <Form.Item name="auth_config_text" label="认证配置 (JSON)" tooltip='Bearer: {"token":"xxx"} | API Key: {"key":"xxx","header":"X-API-Key"} | Basic: {"username":"xxx","password":"xxx"}'>
                  <Input.TextArea rows={3} placeholder='{"token": "your-secret-token"}' style={{ fontFamily: 'monospace', fontSize: 13 }} />
                </Form.Item>
              )}
            </>
          )}
        </Form>
      </Modal>
    </div>
  );
};

export default Connectors;
