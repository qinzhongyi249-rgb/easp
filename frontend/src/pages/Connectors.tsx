import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, App, Tag, Dropdown, Select, Divider, Switch, Alert } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ApiOutlined, MoreOutlined, MinusCircleOutlined, PlusCircleOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Connector } from '../api/connector';
import { connectorApi } from '../api/connector';

const { Title } = Typography;
interface LayoutContext { currentTenant: string; }

const Connectors: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Connector | null>(null);
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

  // 打开编辑弹窗时，解析 headers JSON 为 KV 数组
  const openModal = useCallback((record?: Connector) => {
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
  }, [form]);

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

  const onDelete = async (id: string) => {
    try { await connectorApi.delete(currentTenant, id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (v: string) => <Tag>{v}</Tag> },
      { title: '传输', dataIndex: 'transport_type', key: 'transport_type', width: 100, render: (v: string) => v ? <Tag color={v === 'sse' ? 'blue' : 'green'}>{v}</Tag> : '-' },
      { title: '基础URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
      { title: 'MCP地址', dataIndex: 'mcp_server_url', key: 'mcp_server_url', ellipsis: true, render: (v: string) => v || '-' },
      { title: '认证', dataIndex: 'auth_type', key: 'auth_type', width: 80, render: (v: string) => v ? <Tag color="orange">{v}</Tag> : '-' },
    ] : []),
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'red'}>{v}</Tag> },
    { title: '操作', key: 'action', width: isMobile ? 60 : 150, render: (_: unknown, record: Connector) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => openModal(record) },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDelete(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(record)}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      )
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><ApiOutlined /> 连接器</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>新建连接器</Button>
      </div>
      <Table 
        dataSource={connectors} 
        columns={columns} 
        rowKey="id" 
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={isMobile ? { x: 400 } : undefined}
      />
      <Modal 
        title={editing ? '编辑连接器' : '新建连接器'} 
        open={modalOpen} 
        onOk={onOk} 
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '95%' : 600}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ type: 'mcp', auth_type: 'none', credential_mode: 'static', user_token_header: 'Authorization', user_token_prefix: 'Bearer', user_token_required_sso: true, transport_type: 'streamable_http', headers_kv: [{ key: '', value: '' }] }}>
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
