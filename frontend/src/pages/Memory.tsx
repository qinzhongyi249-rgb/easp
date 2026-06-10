import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, Typography, App, Tabs, Tag, Descriptions, Select, Switch, InputNumber, Space, Popconfirm, Dropdown } from 'antd';
import { PlusOutlined, DatabaseOutlined, UserOutlined, ApartmentOutlined, BulbOutlined, BarChartOutlined, EditOutlined, DeleteOutlined, MoreOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { MemoryPool, UserMemory, Entity, SkillMemory, MemoryStats } from '../api/memory';
import { memoryApi } from '../api/memory';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

const Memory: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [pools, setPools] = useState<MemoryPool[]>([]);
  const [userMemories, setUserMemories] = useState<UserMemory[]>([]);
  const [entities, setEntities] = useState<Entity[]>([]);
  const [skillMemories, setSkillMemories] = useState<SkillMemory[]>([]);
  const [stats, setStats] = useState<MemoryStats | null>(null);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editingPool, setEditingPool] = useState<MemoryPool | null>(null);
  const [activeTab, setActiveTab] = useState('pools');
  const [form] = Form.useForm();
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const [poolsRes, userRes, entityRes, skillRes, statsRes] = await Promise.all([
        memoryApi.listPools(currentTenant),
        memoryApi.listAllUserMemories(currentTenant),
        memoryApi.listEntities(currentTenant),
        memoryApi.listSkillMemories(currentTenant),
        memoryApi.getStats(currentTenant),
      ]);
      setPools(poolsRes.data || []);
      setUserMemories(userRes.data?.memories || []);
      setEntities(entityRes.data?.entities || []);
      setSkillMemories(skillRes.data?.memories || []);
      setStats(statsRes.data || null);
    } catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const openPoolModal = useCallback((record?: MemoryPool) => {
    if (record) {
      setEditingPool(record);
      form.setFieldsValue({
        ...record,
        trigger_rules: record.trigger_rules ? JSON.stringify(JSON.parse(record.trigger_rules), null, 2) : '',
      });
    } else {
      setEditingPool(null);
      form.resetFields();
      form.setFieldsValue({
        type: 'personal',
        purpose: 'conversation',
        priority: 5,
        max_tokens: 0,
        auto_activate: true,
        enabled: true,
      });
    }
    setModalOpen(true);
  }, [form]);

  const onPoolOk = async () => {
    const values = await form.validateFields();
    try {
      // 解析 trigger_rules JSON
      let triggerRules = values.trigger_rules || null;
      if (triggerRules && triggerRules.trim()) {
        try { JSON.parse(triggerRules); } catch { message.error('触发规则必须是合法JSON'); return; }
      }

      const payload: Partial<MemoryPool> = {
        ...values,
        trigger_rules: triggerRules,
      };

      if (editingPool) {
        await memoryApi.updatePool(editingPool.id, payload);
        message.success('更新成功');
      } else {
        await memoryApi.createPool(currentTenant, payload);
        message.success('创建成功');
      }
      setModalOpen(false); form.resetFields(); setEditingPool(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDeletePool = async (id: string) => {
    try { await memoryApi.deletePool(id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  // 记忆池列
  const poolColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (v: string) => {
        const colorMap: Record<string, string> = { personal: 'blue', team: 'green', system: 'purple' };
        return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
      }},
      { title: '用途', dataIndex: 'purpose', key: 'purpose', width: 100, render: (v: string) => {
        const colorMap: Record<string, string> = { conversation: 'cyan', skill: 'orange', knowledge: 'gold' };
        return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
      }},
      { title: '优先级', dataIndex: 'priority', key: 'priority', width: 60 },
      { title: 'Token限制', dataIndex: 'max_tokens', key: 'max_tokens', width: 90, render: (v: number) => v === 0 ? '不限' : v },
      { title: '自动激活', dataIndex: 'auto_activate', key: 'auto_activate', width: 80, render: (v: boolean) => v ? <Tag color="green">是</Tag> : <Tag>否</Tag> },
      { title: '记忆数', dataIndex: 'memory_count', key: 'memory_count', width: 70 },
      { title: '状态', dataIndex: 'enabled', key: 'enabled', width: 60, render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '启用' : '禁用'}</Tag> },
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 60 : 120, render: (_: unknown, record: MemoryPool) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => openPoolModal(record) },
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDeletePool(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openPoolModal(record)}>编辑</Button>
          <Popconfirm title="确认删除?" onConfirm={() => onDeletePool(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      )
    )},
  ];

  // 用户记忆列
  const userMemoryColumns = [
    { title: '类型', dataIndex: 'type', key: 'type', width: 80, render: (v: string) => {
      const colorMap: Record<string, string> = { preference: 'blue', fact: 'green', feedback: 'orange' };
      return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
    }},
    { title: '内容', dataIndex: 'content', key: 'content', ellipsis: true },
    ...(!isMobile ? [
      { title: '用户ID', dataIndex: 'user_id', key: 'user_id', ellipsis: true, width: 120 },
      { title: '访问次数', dataIndex: 'access_count', key: 'access_count', width: 80 },
    ] : []),
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
  ];

  // 实体列
  const entityColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', width: 100, render: (v: string) => {
      const colorMap: Record<string, string> = { tenant: 'purple', user: 'blue', connector: 'cyan', tool: 'green', skill: 'orange' };
      return <Tag color={colorMap[v] || 'default'}>{v}</Tag>;
    }},
    ...(!isMobile ? [
      { title: '引用ID', dataIndex: 'ref_id', key: 'ref_id', ellipsis: true },
    ] : []),
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
  ];

  // 技能记忆列
  const skillMemoryColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
      { title: '分类', dataIndex: 'category', key: 'category', width: 100, render: (v: string) => v ? <Tag>{v}</Tag> : '-' },
      { title: '使用次数', dataIndex: 'usage_count', key: 'usage_count', width: 80 },
    ] : []),
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
  ];

  const tabItems = [
    {
      key: 'pools',
      label: <span><DatabaseOutlined /> 记忆池</span>,
      children: (
        <>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openPoolModal()}>
              新建记忆池
            </Button>
          </div>
          <Table dataSource={pools} columns={poolColumns} rowKey="id" loading={loading}
            size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 500 } : undefined} />
        </>
      ),
    },
    {
      key: 'user',
      label: <span><UserOutlined /> 用户记忆 {stats && <Tag>{stats.total_user_memories}</Tag>}</span>,
      children: (
        <Table dataSource={userMemories} columns={userMemoryColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 400 } : undefined} />
      ),
    },
    {
      key: 'entity',
      label: <span><ApartmentOutlined /> 实体 {stats && <Tag>{stats.total_entities}</Tag>}</span>,
      children: (
        <Table dataSource={entities} columns={entityColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 350 } : undefined} />
      ),
    },
    {
      key: 'skill',
      label: <span><BulbOutlined /> 技能记忆 {stats && <Tag>{stats.total_skill_memories}</Tag>}</span>,
      children: (
        <Table dataSource={skillMemories} columns={skillMemoryColumns} rowKey="id" loading={loading}
          size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 350 } : undefined} />
      ),
    },
    {
      key: 'stats',
      label: <span><BarChartOutlined /> 统计</span>,
      children: stats ? (
        <Descriptions bordered column={isMobile ? 1 : 2} size={isMobile ? 'small' : 'middle'}>
          <Descriptions.Item label="用户记忆">{stats.total_user_memories}</Descriptions.Item>
          <Descriptions.Item label="会话记忆">{stats.total_session_memories}</Descriptions.Item>
          <Descriptions.Item label="实体">{stats.total_entities}</Descriptions.Item>
          <Descriptions.Item label="技能记忆">{stats.total_skill_memories}</Descriptions.Item>
        </Descriptions>
      ) : <Text type="secondary">暂无统计数据</Text>,
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><DatabaseOutlined /> 记忆管理</Title>
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab} items={tabItems} />

      {/* 记忆池编辑弹窗 */}
      <Modal 
        title={editingPool ? '编辑记忆池' : '新建记忆池'} 
        open={modalOpen} 
        onOk={onPoolOk} 
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '95%' : 600}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{
          type: 'personal',
          purpose: 'conversation',
          priority: 5,
          max_tokens: 0,
          auto_activate: true,
          enabled: true,
        }}>
          <Form.Item name="name" label="记忆池名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="如: 工作记忆 / 个人偏好 / 项目知识" />
          </Form.Item>

          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="记忆池的用途说明" />
          </Form.Item>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="type" label="共享类型" style={{ flex: 1 }}>
              <Select options={[
                { value: 'personal', label: '个人' },
                { value: 'team', label: '团队共享' },
                { value: 'system', label: '系统级' },
              ]} />
            </Form.Item>
            <Form.Item name="purpose" label="用途" style={{ flex: 1 }}>
              <Select options={[
                { value: 'conversation', label: '对话记忆' },
                { value: 'skill', label: '技能知识' },
                { value: 'knowledge', label: '知识库' },
              ]} />
            </Form.Item>
          </div>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="priority" label="优先级 (1-10)" style={{ flex: 1 }}>
              <InputNumber min={1} max={10} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="max_tokens" label="最大Token数" tooltip="0表示不限制" style={{ flex: 1 }}>
              <InputNumber min={0} step={100} style={{ width: '100%' }} />
            </Form.Item>
          </div>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="auto_activate" label="自动激活" valuePropName="checked" style={{ flex: 1 }}>
              <Switch />
            </Form.Item>
            <Form.Item name="enabled" label="启用状态" valuePropName="checked" style={{ flex: 1 }}>
              <Switch />
            </Form.Item>
          </div>

          <Form.Item name="trigger_rules" label="触发规则 (JSON)" tooltip='条件激活，如: {"keywords":["工作","会议"]}'>
            <Input.TextArea rows={3} placeholder='{"keywords":["工作"],"context":"对话中提到工作相关内容时激活"}' style={{ fontFamily: 'monospace', fontSize: 13 }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Memory;
