import React, { useState, useEffect } from 'react';
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
  const [importing, setImporting] = useState(false);
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
      await client.post(`/mcp/${currentTenant}/import-openapi`, { connector_id: values.connector_id, spec_url: values.spec_url });
      msg.success('导入成功');
      setImportModalOpen(false); importForm.resetFields(); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; msg.error(e.response?.data?.error || '导入失败'); }
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

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
      { title: '连接器', dataIndex: 'connector_id', key: 'connector_id', render: (v: string) => connectors.find(c => c.id === v)?.name || v },
    ] : []),
    { title: '状态', key: 'status', render: (_: unknown, record: MCPTool) => (
      <Switch size="small" checked={record.enabled} onChange={(checked) => onToggle(record.id, checked)} />
    )},
    { title: '操作', key: 'action', width: isMobile ? 60 : 150, render: (_: unknown, record: MCPTool) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => { setEditing(record); form.setFieldsValue(record); setModalOpen(true); } },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDelete(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(record); form.setFieldsValue(record); setModalOpen(true); }}>编辑</Button>
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
          <Button icon={<ImportOutlined />} onClick={() => setImportModalOpen(true)}>导入OpenAPI</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新建工具</Button>
        </Space>
      </div>
      <Table 
        dataSource={data} 
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
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
      <Modal 
        title="导入OpenAPI" 
        open={importModalOpen} 
        onOk={onImport} 
        onCancel={() => setImportModalOpen(false)}
        confirmLoading={importing}
        width={isMobile ? '90%' : 500}
      >
        <Form form={importForm} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="connector_id" label="连接器" rules={[{ required: true }]}>
            <Select options={connectors.map(c => ({ label: c.name, value: c.id }))} />
          </Form.Item>
          <Form.Item name="spec_url" label="OpenAPI规范URL" rules={[{ required: true }]}>
            <Input placeholder="https://api.example.com/openapi.json" />
          </Form.Item>
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
