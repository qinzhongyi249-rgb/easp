import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Select, Switch, Space, Typography, Popconfirm, Tag, App, Upload } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, ImportOutlined, UploadOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { MCPTool } from '../api/mcpTool';
import type { Connector } from '../api/connector';
import { mcpToolApi } from '../api/mcpTool';
import { connectorApi } from '../api/connector';
import client from '../api/client';

const { Title } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

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

  const onImport = async () => {
    const values = await importForm.validateFields();
    setImporting(true);
    try {
      const res = await client.post(`/tenants/${currentTenant}/mcp/import-openapi`, values);
      const result = res.data as { message: string; total: number; created: number };
      msg.success(`导入成功！共解析 ${result.total} 个接口，创建 ${result.created} 个工具`);
      setImportModalOpen(false);
      importForm.resetFields();
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string; details?: string } } };
      msg.error(e.response?.data?.error || '导入失败');
    } finally {
      setImporting(false);
    }
  };

  const handleFileUpload = (file: File) => {
    const reader = new FileReader();
    reader.onload = (e) => {
      const content = e.target?.result as string;
      importForm.setFieldValue('spec_content', content);
      msg.success('文件已加载');
    };
    reader.readAsText(file);
    return false; // 阻止自动上传
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '方法', dataIndex: 'backend_method', key: 'method', render: (v: string) => <Tag color={v === 'GET' ? 'blue' : v === 'POST' ? 'green' : v === 'PUT' ? 'orange' : 'red'}>{v}</Tag> },
    { title: '路径', dataIndex: 'backend_path', key: 'path', ellipsis: true },
    { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (v: boolean, r: MCPTool) => <Switch checked={v} onChange={(c) => onToggle(r.id, c)} /> },
    { title: '操作', key: 'action', render: (_: unknown, record: MCPTool) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(record); form.setFieldsValue(record); setModalOpen(true); }}>编辑</Button>
        <Popconfirm title="确认删除?" onConfirm={() => { mcpToolApi.delete(currentTenant, record.id).then(load); }}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3}>MCP工具管理</Title>
        <Space>
          <Button icon={<ImportOutlined />} onClick={() => { importForm.resetFields(); setImportModalOpen(true); }}>导入OpenAPI</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新建工具</Button>
        </Space>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} />
      
      {/* 新建/编辑工具 */}
      <Modal title={editing ? '编辑工具' : '新建工具'} open={modalOpen} onOk={onOk} onCancel={() => setModalOpen(false)} width={640}>
        <Form form={form} layout="vertical">
          <Form.Item name="connector_id" label="所属连接器" rules={[{ required: true }]}><Select options={connectors.map(c => ({ value: c.id, label: c.name }))} /></Form.Item>
          <Form.Item name="name" label="工具名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input /></Form.Item>
          <Form.Item name="backend_method" label="HTTP方法" rules={[{ required: true }]}><Select options={['GET','POST','PUT','DELETE','PATCH'].map(m => ({ value: m, label: m }))} /></Form.Item>
          <Form.Item name="backend_path" label="路径" rules={[{ required: true }]}><Input placeholder="/api/v1/users" /></Form.Item>
          <Form.Item name="input_schema" label="参数定义 (JSON Schema)"><TextArea rows={4} placeholder='{"type":"object","properties":{}}' /></Form.Item>
        </Form>
      </Modal>

      {/* 导入OpenAPI */}
      <Modal 
        title="导入OpenAPI文档" 
        open={importModalOpen} 
        onOk={onImport} 
        onCancel={() => setImportModalOpen(false)} 
        width={700}
        confirmLoading={importing}
        okText="导入"
      >
        <Form form={importForm} layout="vertical">
          <Form.Item name="name" label="连接器名称" rules={[{ required: true, message: '请输入连接器名称' }]}>
            <Input placeholder="例如：用户管理系统API" />
          </Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true, message: '请输入API基础URL' }]}>
            <Input placeholder="https://api.example.com" />
          </Form.Item>
          <Form.Item 
            name="spec_content" 
            label="OpenAPI文档内容" 
            rules={[{ required: true, message: '请输入或上传OpenAPI文档' }]}
            extra="支持JSON或YAML格式的OpenAPI/Swagger文档"
          >
            <TextArea rows={10} placeholder='粘贴OpenAPI/Swagger JSON或YAML内容...' />
          </Form.Item>
          <Upload 
            accept=".json,.yaml,.yml" 
            beforeUpload={handleFileUpload} 
            showUploadList={false}
          >
            <Button icon={<UploadOutlined />}>上传文件</Button>
          </Upload>
        </Form>
      </Modal>
    </div>
  );
};

export default MCPTools;
