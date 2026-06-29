import React, { useMemo, useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Select, Switch, Space, Typography, Popconfirm, App, Dropdown, Checkbox, Tag, Spin, Descriptions, Alert, Card, Statistic, Steps, Drawer, Upload } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ImportOutlined, ToolOutlined, MoreOutlined, SearchOutlined, CloudSyncOutlined, ApiOutlined, SafetyCertificateOutlined, CheckCircleOutlined, ExperimentOutlined, EyeOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { MCPTool, MCPToolGovernanceStatus } from '../api/mcpTool';
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
const fmtTime = (value?: string) => value ? new Date(value).toLocaleString() : '-';
const prettyJson = (value?: string) => {
  if (!value) return '无';
  try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
};
const productionState = (tool: MCPTool) => {
  if (isLockedBuiltinTool(tool)) return { label: '内置锁定', color: 'cyan', desc: '系统内置资源不可编辑、停用或删除，后端 API 会强制拦截。' };
  if (!tool.enabled) return { label: '未启用', color: 'default', desc: '工具未启用，不应进入助手生产调用。' };
  if (!['published', 'active'].includes(tool.status || '')) return { label: '未发布', color: 'orange', desc: '需要发布并启用后才建议授权给角色。' };
  if (tool.risk_level === 'high') return { label: '高风险', color: 'red', desc: '高风险工具需要谨慎授权，并关注审计。' };
  return { label: '生产可用', color: 'green', desc: '已发布且启用，可进入角色授权。' };
};

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
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailTool, setDetailTool] = useState<MCPTool | null>(null);
  const [detailGovernance, setDetailGovernance] = useState<MCPToolGovernanceStatus | null>(null);
  const [detailGovernanceLoading, setDetailGovernanceLoading] = useState(false);
  // 测试调用弹窗
  const [testOpen, setTestOpen] = useState(false);
  const [testTool, setTestTool] = useState<MCPTool | null>(null);
  const [testArguments, setTestArguments] = useState<string>('{}');
  const [testUserToken, setTestUserToken] = useState<string>('');
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<string>('');
  // 批量选择删除
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [batchDeleting, setBatchDeleting] = useState(false);

  const handleBatchDelete = async () => {
    if (selectedRowKeys.length === 0) {
      msg.warning('请先选择要删除的工具');
      return;
    }
    setBatchDeleting(true);
    let successCount = 0;
    let failCount = 0;
    for (const key of selectedRowKeys) {
      try {
        await mcpToolApi.delete(currentTenant, key.toString());
        successCount++;
      } catch {
        failCount++;
      }
    }
    msg.success(`批量删除完成: 成功 ${successCount}, 失败 ${failCount}`);
    load();
    setSelectedRowKeys([]);
    setBatchDeleting(false);
  };

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
  const openApiConnectors = connectors.filter(c => ['openapi', 'rest'].includes(c.type));

  const toolSummary = {
    total: data.length,
    published: data.filter(item => item.enabled && ['published', 'active'].includes(item.status || '')).length,
    draft: data.filter(item => !item.status || item.status === 'draft' || item.enabled === false).length,
    highRisk: data.filter(item => item.risk_level === 'high').length,
  };

  const openImportModal = (mode: 'openapi' | 'rest') => {
    importForm.resetFields();
    importForm.setFieldsValue({
      import_mode: mode,
      method: 'GET',
      status: 'draft',
      risk_level: mode === 'rest' ? 'medium' : 'low',
      enabled: false,
      connector_id: mode === 'rest' && openApiConnectors.length > 0 ? openApiConnectors[0].id : undefined,
    });
    setImportModalOpen(true);
  };

  const openDiscoverDropdownItems = mcpConnectors.map(c => ({
    key: c.id,
    label: <span><CloudSyncOutlined /> {c.name}</span>,
    onClick: () => onDiscover(c.id),
  }));

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

  const openDetail = async (tool: MCPTool) => {
    setDetailTool(tool);
    setDetailGovernance(null);
    setDetailOpen(true);
    setDetailGovernanceLoading(true);
    try {
      const res = await mcpToolApi.governanceStatus(currentTenant, tool.id);
      setDetailGovernance(res.data);
    } catch {
      setDetailGovernance(null);
      msg.warning('授权治理状态加载失败');
    } finally {
      setDetailGovernanceLoading(false);
    }
  };

  const openTest = (tool: MCPTool) => {
    setTestTool(tool);
    setTestArguments('{}');
    setTestUserToken('');
    setTestResult('');
    setTestOpen(true);
  };

  const executeTest = async () => {
    if (!testTool || !currentTenant) return;
    setTestLoading(true);
    setTestResult('');
    try {
      let args: any;
      try {
        args = JSON.parse(testArguments);
      } catch (err: unknown) {
        msg.error('参数JSON格式错误');
        return;
      }
      const res = await mcpToolApi.execute(currentTenant, testTool.id, args, testUserToken || undefined);
      setTestResult(JSON.stringify(res.data, null, 2));
      msg.success('调用成功');
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      setTestResult(e.response?.data?.error || '调用失败');
      msg.error('调用失败');
    } finally {
      setTestLoading(false);
    }
  };

  const renderGovernanceStatusTag = (status?: string) => {
    if (status === 'granted') return <Tag color="green">已有角色授权</Tag>;
    if (status === 'unavailable') return <Tag color="orange">当前不可生产执行</Tag>;
    return <Tag color="red">未授权</Tag>;
  };

  const renderRoleTags = (roles?: { id: string; name: string; wildcard?: boolean }[]) => {
    if (!roles || roles.length === 0) return <Text type="secondary">无</Text>;
    return <Space wrap>{roles.map(role => <Tag key={role.id} color={role.wildcard ? 'gold' : 'blue'}>{role.name}{role.wildcard ? ' *' : ''}</Tag>)}</Space>;
  };

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
    { title: '操作', key: 'action', width: isMobile ? 60 : 210, render: (_: unknown, record: MCPTool) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'detail', label: '详情', icon: <EyeOutlined />, onClick: () => openDetail(record) },
          { key: 'test', label: '测试调用', icon: <ExperimentOutlined />, onClick: () => openTest(record) },
          { key: 'edit', label: isLockedBuiltinTool(record) ? '内置工具不可编辑' : '编辑', icon: <EditOutlined />, disabled: isLockedBuiltinTool(record), onClick: () => { if (isLockedBuiltinTool(record)) return; setEditing(record); form.setFieldsValue({ ...record, status: record.status || 'draft' }); setModalOpen(true); } },
          { key: 'delete', label: isLockedBuiltinTool(record) ? '内置工具不可删除' : '删除', icon: <DeleteOutlined />, danger: true, disabled: isLockedBuiltinTool(record), onClick: () => { if (!isLockedBuiltinTool(record)) onDelete(record.id); } },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>详情</Button>
          <Button size="small" icon={<ExperimentOutlined />} onClick={() => openTest(record)}>测试</Button>
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
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><ToolOutlined /> 工具导入与治理</Title>
          <Text type="secondary">把接入源中的 API/MCP 能力导入为 MCP Tool，再统一做生命周期、风险、授权和审计。</Text>
        </div>
        <Space wrap>
          {mcpConnectors.length > 0 && (
            <Dropdown menu={{ items: openDiscoverDropdownItems }}>
              <Button icon={<SearchOutlined />} type="primary" ghost>发现MCP工具</Button>
            </Dropdown>
          )}
          <Button icon={<ImportOutlined />} onClick={() => openImportModal('openapi')}>导入API</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); form.setFieldsValue({ status: 'draft', enabled: false }); setModalOpen(true); }}>新建工具</Button>
        </Space>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="工具是助手真正可调用的生产资源。"
          description="建议默认以草稿/未启用导入，确认 Schema、风险等级和角色授权后再发布。角色权限页只应暴露生产可授权资源。"
        />

        <Card>
          <Steps
            size="small"
            direction={isMobile ? 'vertical' : 'horizontal'}
            items={[
              { title: '选择导入方式', description: 'OpenAPI / REST / MCP发现' },
              { title: '生成工具草稿', description: 'Schema 与路径入库' },
              { title: '测试与发布', description: '确认风险和启用状态' },
              { title: '角色授权', description: '授权后助手才能调用' },
            ]}
          />
        </Card>

        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(4, minmax(0, 1fr))', gap: 12 }}>
          <Card><Statistic title="工具总数" value={toolSummary.total} prefix={<ToolOutlined />} /></Card>
          <Card><Statistic title="生产可用" value={toolSummary.published} prefix={<CheckCircleOutlined />} /></Card>
          <Card><Statistic title="草稿/未启用" value={toolSummary.draft} prefix={<ExperimentOutlined />} /></Card>
          <Card><Statistic title="高风险" value={toolSummary.highRisk} prefix={<SafetyCertificateOutlined />} /></Card>
        </div>

        <Card title="选择导入方式" extra={<Text type="secondary">先导入为草稿，再发布并授权</Text>}>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
            <Card size="small" hoverable onClick={() => openImportModal('openapi')}>
              <Space direction="vertical">
                <Space><ImportOutlined /><Text strong>OpenAPI 批量导入</Text><Tag color="blue">批量</Tag></Space>
                <Text type="secondary">粘贴 OpenAPI JSON/YAML 或填写规范 URL，批量生成工具。</Text>
                <Button size="small" type="primary">导入 OpenAPI</Button>
              </Space>
            </Card>
            <Card size="small" hoverable onClick={() => openImportModal('rest')}>
              <Space direction="vertical">
                <Space><ApiOutlined /><Text strong>REST 单接口</Text><Tag color="purple">精确</Tag></Space>
                <Text type="secondary">手动填写方法、路径和输入 Schema，适合先接一个关键接口。</Text>
                <Button size="small">导入 REST</Button>
              </Space>
            </Card>
            <Card size="small" hoverable>
              <Space direction="vertical">
                <Space><CloudSyncOutlined /><Text strong>MCP Server 发现</Text><Tag color="green">自动</Tag></Space>
                <Text type="secondary">从已配置 MCP Server 发现工具并选择性导入。</Text>
                {mcpConnectors.length > 0 ? (
                  <Dropdown menu={{ items: openDiscoverDropdownItems }}><Button size="small">发现 MCP 工具</Button></Dropdown>
                ) : <Button size="small" disabled>暂无 MCP 接入源</Button>}
              </Space>
            </Card>
          </div>
        </Card>

        <Card title="工具治理" extra={<Text type="secondary">筛选后 {filteredData.length} / 总计 {data.length}</Text>}>
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
          {selectedRowKeys.length > 0 && (
            <Popconfirm
              title={`确认删除选中的 ${selectedRowKeys.length} 个 MCP 工具？此操作不可恢复`}
              onConfirm={handleBatchDelete}
              okButtonProps={{ danger: true }}
            >
              <Button danger icon={<DeleteOutlined />} loading={batchDeleting}>
                批量删除 ({selectedRowKeys.length})
              </Button>
            </Popconfirm>
          )}
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
            rowSelection={{
              selectedRowKeys,
              onChange: (keys) => setSelectedRowKeys(keys),
              getCheckboxProps: (record: MCPTool) => ({
                disabled: isLockedBuiltinTool(record),
              }),
            }}
          />
        </Card>
      </Space>
      <Drawer
        title={`工具治理详情 — ${detailTool?.name || ''}`}
        open={detailOpen}
        onClose={() => { setDetailOpen(false); setDetailGovernance(null); }}
        width={isMobile ? '100%' : 720}
      >
        {detailTool && (() => {
          const state = productionState(detailTool);
          const connector = connectors.find(c => c.id === detailTool.connector_id);
          return (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Alert type={state.color === 'red' ? 'error' : state.color === 'green' ? 'success' : 'info'} showIcon message={state.label} description={state.desc} />
              <Descriptions bordered size="small" column={isMobile ? 1 : 2}>
                <Descriptions.Item label="工具ID">{detailTool.id}</Descriptions.Item>
                <Descriptions.Item label="来源">{isLockedBuiltinTool(detailTool) ? <Tag color="cyan">系统内置</Tag> : <Tag>租户导入</Tag>}</Descriptions.Item>
                <Descriptions.Item label="名称">{detailTool.name}</Descriptions.Item>
                <Descriptions.Item label="连接器">{isLockedBuiltinTool(detailTool) ? 'EASP内置治理工具' : connector?.name || detailTool.connector_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="生命周期">{renderStatusTag(detailTool.status)}</Descriptions.Item>
                <Descriptions.Item label="启用状态">{detailTool.enabled ? <Tag color="green">已启用</Tag> : <Tag>未启用</Tag>}</Descriptions.Item>
                <Descriptions.Item label="锁定状态">{isLockedBuiltinTool(detailTool) ? <Tag color="cyan">内置锁定</Tag> : <Tag>可治理</Tag>}</Descriptions.Item>
                <Descriptions.Item label="风险等级"><Tag color={detailTool.risk_level === 'high' ? 'red' : detailTool.risk_level === 'medium' ? 'orange' : 'green'}>{detailTool.risk_level || '未标注'}</Tag></Descriptions.Item>
                <Descriptions.Item label="方法">{detailTool.backend_method || detailTool.method || '-'}</Descriptions.Item>
                <Descriptions.Item label="路径">{detailTool.backend_path || detailTool.path || '-'}</Descriptions.Item>
                <Descriptions.Item label="创建时间">{fmtTime(detailTool.created_at)}</Descriptions.Item>
                <Descriptions.Item label="更新时间">{fmtTime(detailTool.updated_at)}</Descriptions.Item>
                <Descriptions.Item label="描述" span={isMobile ? 1 : 2}>{detailTool.description || '-'}</Descriptions.Item>
              </Descriptions>
              <Card size="small" title="授权与可执行状态" loading={detailGovernanceLoading}>
                {detailGovernance ? (
                  <Descriptions bordered size="small" column={1}>
                    <Descriptions.Item label="授权状态">{renderGovernanceStatusTag(detailGovernance.authorization_status)}</Descriptions.Item>
                    <Descriptions.Item label="已授权角色数">{detailGovernance.authorized_role_count}</Descriptions.Item>
                    <Descriptions.Item label="授权角色">{renderRoleTags(detailGovernance.authorized_roles)}</Descriptions.Item>
                    <Descriptions.Item label="当前用户可执行">{detailGovernance.current_user_can_execute ? <Tag color="green">是</Tag> : <Tag color="red">否</Tag>}</Descriptions.Item>
                    <Descriptions.Item label="当前用户命中角色">{renderRoleTags(detailGovernance.current_user_granted_roles)}</Descriptions.Item>
                    <Descriptions.Item label="不可执行原因">
                      {detailGovernance.block_reasons?.length ? <Space wrap>{detailGovernance.block_reasons.map(reason => <Tag color="red" key={reason}>{reason}</Tag>)}</Space> : <Text type="secondary">无</Text>}
                    </Descriptions.Item>
                  </Descriptions>
                ) : <Alert type="warning" showIcon message="授权治理状态暂不可用" description="可先检查网络或后端 governance-status 接口。" />}
              </Card>
              <Card size="small" title="Schema / 参数">
                <Text strong>输入 Schema</Text>
                <pre style={{ maxHeight: 220, overflow: 'auto', background: '#f5f5f5', padding: 12, borderRadius: 6 }}>{prettyJson(detailTool.input_schema || detailTool.parameters)}</pre>
              </Card>
              <Card size="small" title="治理建议">
                <Space direction="vertical">
                  <Text>1. 生产调用需同时满足：已发布、已启用、角色已授权。</Text>
                  <Text>2. “当前用户可执行”由后端根据当前登录用户角色、工具生命周期与启用状态实时计算。</Text>
                  <Text>3. 高风险工具建议最小化授权，并通过审计日志追踪调用。</Text>
                  <Text>4. 内置锁定工具由系统保留，不能被普通编辑、停用或删除。</Text>
                </Space>
              </Card>
            </Space>
          );
        })()}
      </Drawer>
      <Modal
        title={editing ? '编辑工具' : '新建工具草稿'}
        open={modalOpen}
        onOk={onOk}
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '90%' : 560}
      >
        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message="新建工具不会自动完成角色授权。"
            description="建议先保持草稿/未启用，测试通过后发布，并在角色管理中授权给需要的运营或嵌入助手用户。"
          />
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input /></Form.Item>
          <Form.Item name="connector_id" label="连接器">
            <Select options={connectors.map(c => ({ label: c.name, value: c.id }))} />
          </Form.Item>
          <Form.Item name="backend_method" label="方法"><Input placeholder="GET / POST / PUT / DELETE" /></Form.Item>
          <Form.Item name="backend_path" label="路径"><Input placeholder="/api/resource" /></Form.Item>
          <Form.Item name="input_schema" label="输入Schema"><TextArea rows={5} placeholder='JSON格式，如 {"type":"object","properties":{"id":{"type":"string"}}}' /></Form.Item>
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
        title="导入 API 生成工具草稿"
        open={importModalOpen}
        onOk={onImport}
        onCancel={() => setImportModalOpen(false)}
        confirmLoading={importing}
        width={isMobile ? '92%' : 760}
      >
        <Form form={importForm} layout="vertical" size={isMobile ? 'middle' : 'large'} initialValues={{ import_mode: 'openapi', method: 'GET', status: 'draft', risk_level: 'medium', enabled: false }}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="导入只负责把 API 转成 MCP Tool，是否能被助手调用由生命周期、启用状态和角色授权共同决定。"
            description="REST 单接口默认草稿且未启用；OpenAPI 批量导入后也建议先检查高风险接口。"
          />
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
              <Form.Item name="spec_url" label="OpenAPI规范URL" tooltip="可填远程 OpenAPI JSON/YAML 地址；也可本地上传文件或直接粘贴内容。" rules={[
                {
                  validator: () => {
                    const content = importForm.getFieldValue('spec_content');
                    const url = importForm.getFieldValue('spec_url');
                    if (!content && !url) {
                      return Promise.reject(new Error('请输入OpenAPI URL、上传文件或粘贴规范内容'));
                    }
                    return Promise.resolve();
                  }
                }
              ]} dependencies={['spec_content']}>
                <Input placeholder="https://api.example.com/openapi.json" />
              </Form.Item>
              <Form.Item label="本地文件上传" tooltip="上传本地 OpenAPI JSON/YAML 文件，上传后内容会自动填入下方规范文本框">
                <Upload
                  accept=".json,.yaml,.yml"
                  beforeUpload={(file) => {
                    const reader = new FileReader();
                    reader.onload = (e) => {
                      const content = e.target?.result as string;
                      importForm.setFieldsValue({ spec_content: content });
                      // 触发重新校验，更新 URL 必填状态
                      importForm.validateFields(['spec_url']);
                    };
                    reader.readAsText(file);
                    return false; // 不上传到服务器，只读取内容
                  }}
                  fileList={[]}
                >
                  <Button>选择文件</Button>
                </Upload>
              </Form.Item>
              <Form.Item name="spec_content" label="OpenAPI规范内容" tooltip="粘贴 OpenAPI JSON/YAML 内容；优先使用此内容。" dependencies={['spec_url']}>
                <TextArea rows={8} placeholder='{"openapi":"3.0.0","info":...,"paths":...}' />
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

      {/* 测试调用弹窗 */}
      <Modal
        title={`测试调用 ${testTool?.name}`}
        open={testOpen}
        onCancel={() => setTestOpen(false)}
        width={isMobile ? '95%' : 700}
        footer={[
          <Button key="cancel" onClick={() => setTestOpen(false)}>关闭</Button>,
          <Button key="execute" type="primary" loading={testLoading} onClick={executeTest}>执行调用</Button>,
        ]}
      >
        <div style={{ marginBottom: 16 }}>
          <Text strong>调用参数 (JSON格式)</Text>
          <TextArea
            rows={8}
            value={testArguments}
            onChange={(e) => setTestArguments(e.target.value)}
            placeholder='{"user_id": "123"}'
            style={{ marginTop: 8 }}
          />
        </div>
        <div style={{ marginBottom: 16 }}>
          <Text strong>用户 Token (可选)</Text>
          <Input.Password
            value={testUserToken}
            onChange={(e) => setTestUserToken(e.target.value)}
            placeholder='当连接器需要透传用户Token时，在此手动录入'
            style={{ marginTop: 8 }}
          />
          <Text type="secondary" style={{ fontSize: 12 }}>
            此功能用于测试 {`credential_mode=user_token`} 的连接器，手动录入业务系统用户登录Token进行透传测试。
          </Text>
        </div>
        {testResult && (
          <div>
            <Text strong>调用结果</Text>
            <pre style={{
              marginTop: 8,
              padding: 12,
              backgroundColor: '#f5f5f5',
              borderRadius: 4,
              maxHeight: 300,
              overflow: 'auto',
              fontSize: 12
            }}>
              {testResult}
            </pre>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default MCPTools;
