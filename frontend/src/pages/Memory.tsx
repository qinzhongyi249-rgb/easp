import React, { useState, useEffect } from 'react';
import { Card, Table, Button, Modal, Form, Input, Select, Space, Typography, Popconfirm, Tag, Row, Col, List, App } from 'antd';
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { MemoryPool, VectorMemory } from '../api/memory';
import { memoryApi } from '../api/memory';

const { Title, Text } = Typography;
const { TextArea } = Input;
interface LayoutContext { currentTenant: string; }

const Memory: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [pools, setPools] = useState<MemoryPool[]>([]);
  const [memories, setMemories] = useState<VectorMemory[]>([]);
  const [selectedPool, setSelectedPool] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [poolModalOpen, setPoolModalOpen] = useState(false);
  const [memModalOpen, setMemModalOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<VectorMemory[]>([]);
  const [poolForm] = Form.useForm();
  const [memForm] = Form.useForm();

  const loadPools = async () => {
    if (!currentTenant) return;
    try { setPools((await memoryApi.listPools(currentTenant)).data || []); }
    catch { setPools([]); }
  };

  const loadMemories = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { setMemories((await memoryApi.listMemories(currentTenant, selectedPool || undefined)).data?.memories || []); }
    catch { setMemories([]); message.error('加载记忆失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { loadPools(); }, [currentTenant]);
  useEffect(() => { loadMemories(); }, [currentTenant, selectedPool]);

  const onCreatePool = async () => {
    const values = await poolForm.validateFields();
    try { await memoryApi.createPool(currentTenant, values); message.success('创建成功'); setPoolModalOpen(false); poolForm.resetFields(); loadPools(); }
    catch { message.error('创建失败'); }
  };

  const onSaveMemory = async () => {
    const values = await memForm.validateFields();
    try { await memoryApi.saveMemory(currentTenant, values); message.success('保存成功'); setMemModalOpen(false); memForm.resetFields(); loadMemories(); }
    catch { message.error('保存失败'); }
  };

  const onSearch = async () => {
    if (!searchQuery.trim()) return;
    try { const res = await memoryApi.searchMemories(currentTenant, searchQuery, selectedPool || undefined); setSearchResults(res.data.memories || []); }
    catch { message.error('搜索失败'); }
  };

  const onDelete = async (id: string) => {
    try { await memoryApi.deleteMemory(currentTenant, id); message.success('删除成功'); loadMemories(); }
    catch { message.error('删除失败'); }
  };

  const memoryColumns = [
    { title: '内容', dataIndex: 'content', key: 'content', ellipsis: true },
    { title: '类型', dataIndex: 'type', key: 'type', render: (v: string) => <Tag>{v}</Tag> },
    { title: '敏感度', dataIndex: 'sensitivity', key: 'sensitivity', render: (v: string) => <Tag color={v === 'high' ? 'red' : v === 'normal' ? 'blue' : 'green'}>{v}</Tag> },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: '操作', key: 'action', render: (_: unknown, record: VectorMemory) => (
      <Popconfirm title="确认删除?" onConfirm={() => onDelete(record.id)}>
        <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
      </Popconfirm>
    )},
  ];

  return (
    <div>
      <Title level={3}>记忆管理</Title>
      <Row gutter={24}>
        <Col span={6}>
          <Card title="记忆池" extra={<Button size="small" icon={<PlusOutlined />} onClick={() => { poolForm.resetFields(); setPoolModalOpen(true); }} />}>
            <List
              dataSource={[{ id: '', name: '全部' } as MemoryPool, ...pools]}
              renderItem={(item) => (
                <List.Item style={{ cursor: 'pointer', background: selectedPool === item.id ? '#e6f7ff' : undefined, padding: '8px 12px', borderRadius: 4 }} onClick={() => setSelectedPool(item.id)}>
                  {item.name}
                </List.Item>
              )}
            />
          </Card>
        </Col>
        <Col span={18}>
          <Card title="记忆条目" extra={
            <Space>
              <Input.Search placeholder="语义搜索..." value={searchQuery} onChange={(e) => setSearchQuery(e.target.value)} onSearch={onSearch} style={{ width: 260 }} />
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { memForm.resetFields(); if (selectedPool) memForm.setFieldValue('pool_id', selectedPool); setMemModalOpen(true); }}>新增记忆</Button>
            </Space>
          }>
            {searchResults.length > 0 && (
              <div style={{ marginBottom: 16 }}>
                <Text strong>搜索结果 ({searchResults.length})</Text>
                <Button size="small" style={{ marginLeft: 8 }} onClick={() => setSearchResults([])}>清除</Button>
                <Table dataSource={searchResults} columns={memoryColumns} rowKey="id" size="small" pagination={false} style={{ marginTop: 8 }} />
              </div>
            )}
            <Table dataSource={memories} columns={memoryColumns} rowKey="id" loading={loading} />
          </Card>
        </Col>
      </Row>

      <Modal title="新建记忆池" open={poolModalOpen} onOk={onCreatePool} onCancel={() => setPoolModalOpen(false)}>
        <Form form={poolForm} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>

      <Modal title="新增记忆" open={memModalOpen} onOk={onSaveMemory} onCancel={() => setMemModalOpen(false)}>
        <Form form={memForm} layout="vertical">
          <Form.Item name="pool_id" label="记忆池"><Select allowClear options={pools.map(p => ({ value: p.id, label: p.name }))} placeholder="选择记忆池（可选）" /></Form.Item>
          <Form.Item name="content" label="内容" rules={[{ required: true }]}><TextArea rows={4} /></Form.Item>
          <Form.Item name="type" label="类型" initialValue="fact"><Select options={[{ value: 'fact', label: '事实' }, { value: 'preference', label: '偏好' }, { value: 'workflow', label: '工作流' }, { value: 'skill', label: '技能' }]} /></Form.Item>
          <Form.Item name="sensitivity" label="敏感度" initialValue="normal"><Select options={[{ value: 'low', label: '低' }, { value: 'normal', label: '普通' }, { value: 'high', label: '高' }]} /></Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Memory;
