import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Select, Space, Typography, Popconfirm, Tag, App } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SyncOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Connector } from '../api/connector';
import { connectorApi } from '../api/connector';

const { Title } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

const Connectors: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [data, setData] = useState<Connector[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Connector | null>(null);
  const [form] = Form.useForm();

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { setData((await connectorApi.list(currentTenant)).data || []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onOk = async () => {
    const values = await form.validateFields();
    try {
      if (editing) { await connectorApi.update(currentTenant, editing.id, values); message.success('更新成功'); }
      else { await connectorApi.create(currentTenant, values); message.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onSync = async (id: string) => {
    try { await connectorApi.syncOpenAPI(currentTenant, id); message.success('同步成功'); load(); }
    catch { message.error('同步失败'); }
  };

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (v: string) => <Tag>{v}</Tag> },
    { title: 'Base URL', dataIndex: 'base_url', key: 'base_url', ellipsis: true },
    { title: '认证类型', dataIndex: 'auth_type', key: 'auth_type' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'red'}>{v}</Tag> },
    { title: '操作', key: 'action', render: (_: unknown, record: Connector) => (
      <Space>
        <Button size="small" icon={<SyncOutlined />} onClick={() => onSync(record.id)}>同步</Button>
        <Button size="small" icon={<EditOutlined />} onClick={() => { setEditing(record); form.setFieldsValue(record); setModalOpen(true); }}>编辑</Button>
        <Popconfirm title="确认删除?" onConfirm={() => { connectorApi.delete(currentTenant, record.id).then(load); }}>
          <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
        </Popconfirm>
      </Space>
    )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3}>连接器管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setModalOpen(true); }}>新建连接器</Button>
      </div>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} />
      <Modal title={editing ? '编辑连接器' : '新建连接器'} open={modalOpen} onOk={onOk} onCancel={() => setModalOpen(false)} width={640}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true }]}><Select options={[{ value: 'rest', label: 'REST API' }, { value: 'graphql', label: 'GraphQL' }, { value: 'grpc', label: 'gRPC' }]} /></Form.Item>
          <Form.Item name="base_url" label="Base URL" rules={[{ required: true }]}><Input placeholder="https://api.example.com" /></Form.Item>
          <Form.Item name="auth_type" label="认证类型" rules={[{ required: true }]}><Select options={[{ value: 'none', label: '无认证' }, { value: 'bearer', label: 'Bearer Token' }, { value: 'api_key', label: 'API Key' }, { value: 'basic', label: 'Basic Auth' }, { value: 'oauth2', label: 'OAuth2' }]} /></Form.Item>
          <Form.Item name="auth_config" label="认证配置 (JSON)"><TextArea rows={3} placeholder='{"token": "xxx"}' /></Form.Item>
          <Form.Item name="openapi_spec" label="OpenAPI规范 (JSON/YAML)"><TextArea rows={6} placeholder="粘贴OpenAPI规范..." /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Connectors;
