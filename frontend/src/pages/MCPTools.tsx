import React, { useMemo, useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Select, Switch, Space, Typography, Popconfirm, App, Dropdown, Checkbox, Tag, Spin, Descriptions, Alert } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ImportOutlined, ToolOutlined, MoreOutlined, SearchOutlined, CloudSyncOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { MCPTool } from '../api/mcpTool';
import type { Connector } from '../api/connector';
import { mcpToolApi } from '../api/mcpTool';
import { connectorApi } from '../api/connector';
import client from '../api/client';

const { Title, Text } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

const STATUS_META: Record<string, { label: string; color: string }> = {
  draft: { label: '草稿', color: 'blue' },
  testing: { label: '测试中', color: 'gold' },
  published: { label: '已发布', color: 'green' },
  disabled: { label: '已停用', color: 'default' },
  active: { label: '已发布(旧)', color: 'green' },
  archived: { label: '已停用(旧)', color: 'default' },
};

const renderStatusTag = (status?: string) => {
  const meta = STATUS_META[status || ''] || { label: status || '草稿', color: 'default' };
  return <Tag color={meta.color}>{meta.label}</Tag>;
};

interface MCPToolFilters {
  keyword: string;
  status?: string;
  enabled?: boolean;
  connector_id?: string;
  risk_level?: string;
}

const normalizeText = (value?: string | null) => (value || '').toLowerCase();
const isLockedBuiltinTool = (tool: MCPTool) => Boolean(tool.is_builtin || tool.locked);

interface DiscoveredTool {
  name: string;
  description?: string;
  inputSchema?: unknown;
}

interface ServerInfo {
  name: string;
  version?: string;
}

const MCPTools: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message: msg } = App.useApp();
  const [data, setData] = useState<MCPTool[]>([]);
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [importModalOpen, setImportModalOpen] = useState(false);
  const [editing, setEditing] = useState<MCPTool | null>(null);
  const [form] = Form.useForm();
  const [importForm] = Form.useForm();
  const [filters, setFilters] = useState<MCPToolFilters>({ keyword: '' });
  const [importing, setImporting] = useState(false);
  const importMode = Form.useWatch('import_mode', importForm);
  const isMobile = window.innerWidth < 768;

  // MCP 发现状态
  const [discoverModalOpen, setDiscoverModalOpen] = useState(false);
  const [discoverLoading, setDiscoverLoading] = useState(false);
  const [discoveredTools, setDiscoveredTools] = useState<DiscoveredTool[]>([]);
  const [discoveredServerInfo, setDiscoveredServerInfo] = useState<ServerInfo | null>(null);
  const [selectedTools, setSelectedTools] = useState<string[]>([]);
  const [discoverConnectorId, setDiscoverConnectorId] = useState<string>('');
  const [importingMCP, setImportingMCP] = useState(false);

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { const [t, c] = await Promise.all([mcpToolApi.list(currentTenant), connectorApi.list(currentTenant)]); setData(t.data || []); setConnectors(c.data || []); }
    catch { msg.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onOk = async () => {
    const values = await form.validateFields();
    try {
      if (editing) { await mcpToolApi.update(currentTenant, editing.id, values); msg.success('更新成功'); }
      else { await mcpToolApi.create(currentTenant, values); msg.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; msg.error(e.response?.data?.error || '操作失败'); }
  };

  const onToggle = async (id: string, enabled: boolean) => {
    try { await mcpToolApi.toggleEnabled(currentTenant, id, enabled); msg.success(enabled ? '已启用' : '已禁用'); load(); }
    catch { msg.error('操作失败'); }
  };

  const onDelete = async (id: string) => {
    try { await mcpToolApi.delete(currentTenant, id); msg.success('删除成功'); load(); }
    catch { msg.error('删除失败'); }
  };

  const onImport = async () => {
    const values = await importForm.validateFields();
    setImporting(true);
    try {
      if (values.import_mode === 'rest' && values.input_schema?.trim()) {
        JSON.parse(values.input_schema);
      }
      const payload = values.import_mode === 'rest'
        ? {
            connector_id: values.connector_id,
            name: values.name,
            method: values.method,
            api_path: values.api_path,
            description: values.description || '',
            input_schema: values.input_schema || undefined,
            status: values.status || 'draft',
            risk_level: values.risk_level || 'medium',
            enabled: values.enabled ?? false,
          }
        : {
            connector_id: values.connector_id,
            spec_url: values.spec_url,
            spec_content: values.spec_content || undefined,
          };
      await client.post(`/mcp/${currentTenant}/import-openapi`, payload);
      msg.success(values.import_mode === 'rest' ? 'RESTful API 导入成功' : 'OpenAPI 导入成功');
      setImportModalOpen(false); importForm.resetFields(); load();
    } catch (err: unknown) {
      if (err instanceof SyntaxError) { msg.error('输入 Schema 不是合法 JSON'); return; }
      const e = err as { response?: { data?: { error?: string } } }; msg.error(e.response?.data?.error || '导入失败');
    }
    finally { setImporting(false); }
  };

  // MCP 发现
  const onDiscover = async (connectorId: string) => {
    setDiscoverConnectorId(connectorId);
    setDiscoverLoading(true);
    setDiscoveredTools([]);
    setDiscoveredServerInfo(null);
    setSelectedTools([]);
    setDiscoverModalOpen(true);

    try {
      const res = await client.post(`/tenants/${currentTenant}/connectors/${connectorId}/discover-mcp`);
      const d = res.data as { server_info?: ServerInfo; tools?: DiscoveredTool[]; total?: number };
      setDiscoveredTools(d.tools || []);
      setDiscoveredServerInfo(d.server_info || null);
      setSelectedTools((d.tools || []).map(t => t.name)); // 默认全选
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string; details?: string } } };
      msg.error(e.response?.data?.error || 'MCP工具发现失败');
      setDiscoveredTools([]);
    } finally {
      setDiscoverLoading(false);
    }
  };

  // MCP 导入
  const onImportMCP = async () => {
    if (selectedTools.length === 0) {
      msg.warning('请至少选择一个工具');
      return;
    }
    setImportingMCP(true);
    try {
      const res = await client.post(`/tenants/${currentTenant}/connectors/${discoverConnectorId}/import-mcp-tools`, {
        tool_names: selectedTools,
      });
      const d = res.data as { created?: number; updated?: number; server_info?: ServerInfo };
      msg.success(`导入完成: 新增 ${d.created || 0}, 更新 ${d.updated || 0}`);
      setDiscoverModalOpen(false); load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      msg.error(e.response?.data?.error || '导入失败');
    } finally {
      setImportingMCP(false);
    }
  };

  // 获取有 MCP Server URL 的连接器
  const mcpConnectors = connectors.filter(c => c.mcp_server_url);

  const filteredData = useMemo(() => {
    const keyword = normalizeText(filters.keyword).trim();
    return data.filter((tool) => {
      if (keyword) {
        const haystack = [tool.name, tool.description, tool.method, tool.path, tool.backend_method, tool.backend_path]
          .map(normalizeText)
          .join(' ');
        if (!haystack.includes(keyword)) return false;
      }
      if (filters.status && tool.status !== filters.status) return false;
      if (filters.enabled !== undefined && tool.enabled !== filters.enabled) return false;
      if (filters.connector_id && tool.connector_id !== filters.connector_id) return false;
      if (filters.risk_level && tool.risk_level !== filters.risk_level) return false;
      return true;
    });
  }, [data, filters]);

  const hasActiveFilters = Boolean(
    filters.keyword || filters.status || filters.enabled !== undefined || filters.connector_id || filters.risk_level
  );

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, record: MCPTool) => (
      <Space size={4} wrap>
        <span>{v}</span>
        {isLockedBuiltinTool(record) && <Tag color="cyan">内置锁定</Tag>}
      </Space>
    )},
    ...(!isMobile ? [
      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
      { title: '连接器', dataIndex: 'connector_id', key: 'connector_id', render: (v: string, record: MCPTool) => isLockedBuiltinTool(record) ? <Tag color="cyan">EASP内置治理工具</Tag> : (connectors.find(c => c.id === v)?.name || v) },
      { title: '生命周期', dataIndex: 'status', key: 'lifecycle_status', width: 110, render: (v: string) => renderStatusTag(v) },
    ] : []),
    { title: '启用', key: 'enabled', render: (_: unknown, record: MCPTool) => (
      <Switch size="small" checked={record.enabled} disabled={isLockedBuiltinTool(record)} onChange={(checked) => onToggle(record.id, checked)} />
    )},
    { title: '操作', key: 'action', width: isMobile ? 60 : 150, render: (_: unknown, record: MCPTool) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: isLockedBuiltinTool(record) ? '内置工具不可编辑' : '编辑', icon: <EditOutlined />, disabled: isLockedBuiltinTool(record), onClick: () => { if (isLockedBuiltinTool(record)) return; setEditing(record); form.setFieldsValue({ ...record, status: record.status || 'draft' }); setModalOpen(true); } },
          { key: 'delete', label: isLockedBuiltinTool(record) ? '内置工具不可删除' : '删除', icon: <DeleteOutlined />, danger: true, disabled: isLockedBuiltinTool(record), onClick: () => { if (!isLockedBuiltinTool(record)) onDelete(record.id); } },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} disabled={isLockedBuiltinTool(record)} onClick={() => { setEditing(record); form.setFieldsValue({ ...record, status: record.status || 'draft' }); setModalOpen(true); }}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(record.id)} disabled={isLockedBuiltinTool(record)}>
            <Button size="small" danger icon={<DeleteOutlined />} disabled={isLockedBuiltinTool(record)}>删除</Button>
          </Popconfirm>
        </Space>
      )
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><ToolOutlined /> MCP工具</Title>
        <Space wrap>
          {mcpConnectors.length > 0 && (
            <Dropdown menu={{
              items: mcpConnectors.map(c => ({
                key: c.id,
                label: <span><CloudSyncOutlined /> {c.name}</span>,
                onClick: () => onDiscover(c.id),
              })),
            }}>
              <Button icon={<SearchOutlined />} type="primary" ghost>发现MCP工具</Button>
            </Dropdown>
          )}
          <Button icon={<ImportOutlined />} onClick={() => { importForm.resetFields(); importForm.setFieldsValue({ import_mode: 'openapi', method: 'GET', status: 'draft', risk_level: 'medium', enabled: false }); setImportModalOpen(true); }}>导入API</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); form.setFieldsValue({ status: 'draft', enabled: true }); setModalOpen(true); }}>新建工具</Button>
        </Space>
      </div>
      <div style={{ marginBottom: 16, padding: 12, background: '#fafafa', border: '1px solid #f0f0f0', borderRadius: 8 }}>
        <Space wrap size="middle" style={{ width: '100%' }}>
          <Input.Search
            allowClear
            placeholder="搜索名称/描述/方法/路径"
            style={{ width: isMobile ? '100%' : 260 }}
            value={filters.keyword}
            onChange={(e) => setFilters(prev => ({ ...prev, keyword: e.target.value }))}
          />
          <Select
            allowClear
            placeholder="生命周期"
            style={{ width: 130 }}
            value={filters.status}
            onChange={(value) => setFilters(prev => ({ ...prev, status: value }))}
            options={[
              { value: 'draft', label: '草稿' },
              { value: 'testing', label: '测试中' },
              { value: 'published', label: '已发布' },
              { value: 'disabled', label: '已停用' },
              { value: 'active', label: '已发布(旧)' },
              { value: 'archived', label: '已停用(旧)' },
            ]}
          />
          <Select
            allowClear
            placeholder="启用状态"
            style={{ width: 120 }}
            value={filters.enabled}
            onChange={(value) => setFilters(prev => ({ ...prev, enabled: value }))}
            options={[
              { value: true, label: '已启用' },
              { value: false, label: '已禁用' },
            ]}
          />
          <Select
            allowClear
            showSearch
            optionFilterProp="label"
            placeholder="连接器"
            style={{ width: isMobile ? '100%' : 180 }}
            value={filters.connector_id}
            onChange={(value) => setFilters(prev => ({ ...prev, connector_id: value }))}
            options={connectors.map(c => ({ label: c.name, value: c.id }))}
          />
          <Select
            allowClear
            placeholder="风险等级"
            style={{ width: 120 }}
            value={filters.risk_level}
            onChange={(value) => setFilters(prev => ({ ...prev, risk_level: value }))}
            options={[
              { value: 'low', label: '低风险' },
              { value: 'medium', label: '中风险' },
              { value: 'high', label: '高风险' },
            ]}
          />
          <Button disabled={!hasActiveFilters} onClick={() => setFilters({ keyword: '' })}>重置</Button>
          <Text type="secondary">共 {filteredData.length} / {data.length} 个</Text>
        </Space>
      </div>
      <Table
        dataSource={filteredData}
        columns={columns}
        rowKey="id"
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={isMobile ? { x: 300 } : undefined}
      />
      <Modal
        title={editing ? '编辑工具' : '新建工具'}
        open={modalOpen}
        onOk={onOk}
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '90%' : 500}
      >
        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input /></Form.Item>
          <Form.Item name="connector_id" label="连接器">
            <Select options={connectors.map(c => ({ label: c.name, value: c.id }))} />
          </Form.Item>
          <Form.Item name="method" label="方法"><Input placeholder="GET / POST / PUT / DELETE" /></Form.Item>
          <Form.Item name="endpoint" label="端点"><Input placeholder="/api/resource" /></Form.Item>
          <Form.Item name="parameters" label="参数"><TextArea rows={3} placeholder='JSON格式' /></Form.Item>
          <Form.Item name="status" label="生命周期" initialValue="draft">
            <Select options={[
              { value: 'draft', label: '草稿' },
              { value: 'testing', label: '测试中' },
              { value: 'published', label: '已发布' },
              { value: 'disabled', label: '已停用' },
            ]} />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
      <Modal
        title="导入API生成MCP工具"
        open={importModalOpen}
        onOk={onImport}
        onCancel={() => setImportModalOpen(false)}
        confirmLoading={importing}
        width={isMobile ? '90%' : 640}
      >
        <Form form={importForm} layout="vertical" size={isMobile ? 'middle' : 'large'} initialValues={{ import_mode: 'openapi', method: 'GET', status: 'draft', risk_level: 'medium', enabled: false }}>
          <Form.Item name="import_mode" label="导入类型" rules={[{ required: true }]}>
            <Select options={[
              { value: 'openapi', label: 'OpenAPI 文档' },
              { value: 'rest', label: 'RESTful 单接口' },
            ]} />
          </Form.Item>
          <Form.Item name="connector_id" label="连接器" rules={[{ required: true, message: '请选择连接器' }]}>
            <Select showSearch optionFilterProp="label" options={connectors.map(c => ({ label: c.name, value: c.id }))} />
          </Form.Item>

          {importMode !== 'rest' && (
            <>
              <Form.Item name="spec_url" label="OpenAPI规范URL" tooltip="可填远程 OpenAPI JSON/YAML 地址；如填写下方内容，可留空。" rules={[{ required: !importForm.getFieldValue('spec_content'), message: '请输入OpenAPI URL或规范内容' }]}>
                <Input placeholder="https://api.example.com/openapi.json" />
              </Form.Item>
              <Form.Item name="spec_content" label="OpenAPI规范内容" tooltip="粘贴 OpenAPI JSON/YAML 内容；优先使用此内容。">
                <TextArea rows={5} placeholder='{"openapi":"3.0.0","info":...,"paths":...}' />
              </Form.Item>
            </>
          )}

          {importMode === 'rest' && (
            <>
              <Form.Item name="name" label="工具名称" rules={[{ required: true, message: '请输入工具名称' }]}>
                <Input placeholder="update_user / get_order" />
              </Form.Item>
              <div style={{ display: 'flex', gap: 12 }}>
                <Form.Item name="method" label="请求方法" rules={[{ required: true }]} style={{ width: 150 }}>
                  <Select options={['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map(m => ({ value: m, label: m }))} />
                </Form.Item>
                <Form.Item name="api_path" label="API路径" rules={[{ required: true, message: '请输入API路径' }]} style={{ flex: 1 }}>
                  <Input placeholder="/api/users/{id}" />
                </Form.Item>
              </div>
              <Form.Item name="description" label="描述">
                <Input placeholder="接口用途说明" />
              </Form.Item>
              <Form.Item name="input_schema" label="输入 Schema(JSON)" tooltip="可选；留空时后端使用空 object schema。">
                <TextArea rows={5} placeholder='{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}' />
              </Form.Item>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16 }}
                message="默认以草稿且未启用导入，避免新接口直接进入生产可调用范围；确认 Schema、风险和权限后再发布启用。"
              />
              <div style={{ display: 'flex', gap: 12, flexWrap: 'wrap' }}>
                <Form.Item name="status" label="生命周期" rules={[{ required: true }]} style={{ minWidth: 160, flex: 1 }}>
                  <Select options={[
                    { value: 'draft', label: '草稿' },
                    { value: 'testing', label: '测试中' },
                    { value: 'published', label: '已发布' },
                    { value: 'disabled', label: '已停用' },
                  ]} />
                </Form.Item>
                <Form.Item name="risk_level" label="风险等级" rules={[{ required: true }]} style={{ minWidth: 160, flex: 1 }}>
                  <Select options={[
                    { value: 'low', label: '低' },
                    { value: 'medium', label: '中' },
                    { value: 'high', label: '高' },
                  ]} />
                </Form.Item>
                <Form.Item name="enabled" label="导入后启用" valuePropName="checked" tooltip="草稿建议保持关闭，发布后再启用。" style={{ minWidth: 120 }}>
                  <Switch />
                </Form.Item>
              </div>
            </>
          )}
        </Form>
      </Modal>

      {/* MCP 发现弹窗 */}
      <Modal
        title="发现MCP工具"
        open={discoverModalOpen}
        onCancel={() => setDiscoverModalOpen(false)}
        width={isMobile ? '95%' : 650}
        footer={discoveredTools.length > 0 ? [
          <Button key="cancel" onClick={() => setDiscoverModalOpen(false)}>取消</Button>,
          <Button key="import" type="primary" loading={importingMCP} onClick={onImportMCP}
            disabled={selectedTools.length === 0}>
            导入已选 ({selectedTools.length})
          </Button>,
        ] : undefined}
      >
        {discoverLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin size="large" />
            <div style={{ marginTop: 16 }}><Text type="secondary">正在连接MCP Server发现工具...</Text></div>
          </div>
        ) : discoveredTools.length === 0 ? (
          <Alert type="warning" message="未发现可用工具" showIcon />
        ) : (
          <>
            {discoveredServerInfo && (
              <Descriptions size="small" column={2} style={{ marginBottom: 16 }}>
                <Descriptions.Item label="服务名">{discoveredServerInfo.name}</Descriptions.Item>
                <Descriptions.Item label="版本">{discoveredServerInfo.version || '-'}</Descriptions.Item>
              </Descriptions>
            )}
            <div style={{ marginBottom: 8 }}>
              <Checkbox
                checked={selectedTools.length === discoveredTools.length}
                indeterminate={selectedTools.length > 0 && selectedTools.length < discoveredTools.length}
                onChange={(e) => setSelectedTools(e.target.checked ? discoveredTools.map(t => t.name) : [])}
              >
                全选 ({discoveredTools.length} 个工具)
              </Checkbox>
            </div>
            <div style={{ maxHeight: 400, overflow: 'auto', border: '1px solid #f0f0f0', borderRadius: 8 }}>
              {discoveredTools.map((tool) => (
                <div key={tool.name} style={{
                  padding: '10px 12px',
                  borderBottom: '1px solid #f0f0f0',
                  display: 'flex', alignItems: 'flex-start', gap: 8,
                  background: selectedTools.includes(tool.name) ? '#f6ffed' : '#fff',
                }}>
                  <Checkbox
                    checked={selectedTools.includes(tool.name)}
                    onChange={(e) => {
                      if (e.target.checked) setSelectedTools(prev => [...prev, tool.name]);
                      else setSelectedTools(prev => prev.filter(n => n !== tool.name));
                    }}
                    style={{ marginTop: 2 }}
                  />
                  <div style={{ flex: 1 }}>
                    <div><Text strong>{tool.name}</Text></div>
                    {tool.description && <div><Text type="secondary" style={{ fontSize: 12 }}>{tool.description}</Text></div>}
                    {tool.inputSchema !== undefined && tool.inputSchema !== null && (
                      <div style={{ marginTop: 4 }}>
                        <Tag color="blue" style={{ fontSize: 11 }}>有参数定义</Tag>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </>
        )}
      </Modal>
    </div>
  );
};

export default MCPTools;
