import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, App, Tabs, Tag, Select } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SafetyCertificateOutlined, TeamOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Role } from '../api/role';
import { roleApi } from '../api/role';
import { useAuth } from '../contexts/AuthContext';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

// 工具权限选项
const TOOL_OPTIONS = [
  { label: '连接器管理', value: 'connectors', desc: '创建、编辑、删除连接器' },
  { label: 'MCP工具管理', value: 'mcp-tools', desc: '管理MCP工具配置' },
  { label: '技能管理', value: 'skills', desc: '创建和执行技能' },
  { label: '记忆管理', value: 'memory', desc: '管理记忆池和向量记忆' },
  { label: '模型配置', value: 'model-config', desc: '配置AI模型和提供商' },
  { label: '用户管理', value: 'users', desc: '管理租户下的用户' },
  { label: '角色管理', value: 'roles', desc: '管理租户下的角色' },
  { label: '审计日志', value: 'audit-logs', desc: '查看操作审计日志' },
  { label: 'SSO配置', value: 'sso-config', desc: '配置单点登录' },
];

// 数据范围选项
const DATA_SCOPE_OPTIONS = [
  { label: '全部数据', value: 'all', desc: '可访问租户下所有数据' },
  { label: '本部门数据', value: 'department', desc: '仅可访问本部门数据' },
  { label: '仅个人数据', value: 'self', desc: '仅可访问自己创建的数据' },
];

// 限流预设
const RATE_LIMIT_PRESETS = [
  { label: '不限制', value: '' },
  { label: '100次/小时', value: '100/hour' },
  { label: '500次/小时', value: '500/hour' },
  { label: '1000次/小时', value: '1000/hour' },
  { label: '5000次/小时', value: '5000/hour' },
  { label: '自定义', value: 'custom' },
];

const Roles: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { isAdmin } = useAuth();
  const { message } = App.useApp();
  const [tenantRoles, setTenantRoles] = useState<Role[]>([]);
  const [systemRoles, setSystemRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Role | null>(null);
  const [form] = Form.useForm();
  const [rateLimitMode, setRateLimitMode] = useState<string>('');

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const res = await roleApi.list(currentTenant);
      const data = res.data as { tenant_roles?: Role[]; system_roles?: Role[] } | Role[];
      if (Array.isArray(data)) {
        setTenantRoles(data);
        setSystemRoles([]);
      } else {
        setTenantRoles(data.tenant_roles || []);
        setSystemRoles(data.system_roles || []);
      }
    } catch {
      message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onOk = async () => {
    const values = await form.validateFields();
    // tools 从数组转为 JSON 字符串
    if (values.tools && Array.isArray(values.tools)) {
      values.tools = JSON.stringify(values.tools);
    }
    try {
      if (editing) { await roleApi.update(currentTenant, editing.id, values); message.success('更新成功'); }
      else { await roleApi.create(currentTenant, values); message.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败'); }
  };

  const onDelete = async (id: string) => {
    try { await roleApi.delete(currentTenant, id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const openModal = (record?: Role) => {
    if (record) {
      setEditing(record);
      // 解析 tools JSON 字符串为数组
      let toolsArray: string[] = [];
      if (record.tools) {
        try { toolsArray = JSON.parse(record.tools); } catch { toolsArray = []; }
      }
      form.setFieldsValue({
        ...record,
        tools: toolsArray,
      });
      // 设置限流模式
      const preset = RATE_LIMIT_PRESETS.find(p => p.value === record.rate_limit);
      if (preset && preset.value) {
        setRateLimitMode(preset.value);
      } else if (record.rate_limit) {
        setRateLimitMode('custom');
      } else {
        setRateLimitMode('');
      }
    } else {
      setEditing(null);
      form.resetFields();
      setRateLimitMode('');
    }
    setModalOpen(true);
  };

  const tenantColumns = [
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, r: Role) => (
      <span>{v} {r.is_default && <Tag color="blue">默认</Tag>}</span>
    )},
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '工具权限', dataIndex: 'tools', key: 'tools', render: (v: string) => {
      if (!v) return <Text type="secondary">-</Text>;
      try {
        const tools: string[] = JSON.parse(v);
        return (
          <Space size={[0, 4]} wrap>
            {tools.map(t => {
              const opt = TOOL_OPTIONS.find(o => o.value === t);
              return <Tag key={t} color="blue">{opt?.label || t}</Tag>;
            })}
          </Space>
        );
      } catch { return <Text type="secondary">{v}</Text>; }
    }},
    { title: '限流', dataIndex: 'rate_limit', key: 'rate_limit', render: (v: string) => v || <Text type="secondary">不限</Text> },
    { title: '数据范围', dataIndex: 'data_scope', key: 'data_scope', render: (v: string) => {
      const opt = DATA_SCOPE_OPTIONS.find(o => o.value === v);
      return opt ? <Tag>{opt.label}</Tag> : (v || <Text type="secondary">-</Text>);
    }},
    { title: '操作', key: 'action', render: (_: unknown, record: Role) => (
      <Space>
        <Button size="small" icon={<EditOutlined />} onClick={() => openModal(record)}>编辑</Button>
        {!record.is_default && (
          <Popconfirm title="确认删除?" onConfirm={() => onDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        )}
      </Space>
    )},
  ];

  const systemColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '工具权限', dataIndex: 'tools', key: 'tools', ellipsis: true },
    { title: '限流', dataIndex: 'rate_limit', key: 'rate_limit' },
    { title: '数据范围', dataIndex: 'data_scope', key: 'data_scope' },
    { title: '状态', key: 'status', render: () => <Tag color="green">系统级</Tag> },
  ];

  const tabItems = [
    {
      key: 'tenant',
      label: <span><TeamOutlined /> 租户角色</span>,
      children: (
        <>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>新建角色</Button>
          </div>
          <Table dataSource={tenantRoles} columns={tenantColumns} rowKey="id" loading={loading} pagination={false} />
        </>
      ),
    },
    ...(isAdmin ? [{
      key: 'system',
      label: <span><SafetyCertificateOutlined /> 系统角色</span>,
      children: (
        <Table dataSource={systemRoles} columns={systemColumns} rowKey="id" loading={loading} pagination={false} />
      ),
    }] : []),
  ];

  return (
    <div>
      <Title level={3}>角色管理</Title>
      <Tabs items={tabItems} defaultActiveKey="tenant" />
      <Modal 
        title={editing ? '编辑角色' : '新建角色'} 
        open={modalOpen} 
        onOk={onOk} 
        onCancel={() => setModalOpen(false)}
        width={600}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input placeholder="如：开发者、运维人员、访客" />
          </Form.Item>
          <Form.Item name="description" label="角色描述">
            <Input.TextArea rows={2} placeholder="描述该角色的职责和权限范围" />
          </Form.Item>
          <Form.Item name="tools" label="工具权限" extra="选择该角色可以使用的功能模块">
            <Select
              mode="multiple"
              placeholder="选择可使用的功能模块"
              options={TOOL_OPTIONS.map(o => ({
                label: (
                  <div>
                    <div>{o.label}</div>
                    <Text type="secondary" style={{ fontSize: 12 }}>{o.desc}</Text>
                  </div>
                ),
                value: o.value,
              }))}
              maxTagCount="responsive"
              allowClear
            />
          </Form.Item>
          <Form.Item label="访问频率限制">
            <Space>
              <Select
                value={rateLimitMode}
                onChange={(v) => {
                  setRateLimitMode(v);
                  if (v !== 'custom') {
                    form.setFieldValue('rate_limit', v);
                  }
                }}
                style={{ width: 160 }}
                options={RATE_LIMIT_PRESETS}
              />
              {rateLimitMode === 'custom' && (
                <Form.Item name="rate_limit" noStyle>
                  <Input placeholder="如：200/hour, 50/minute" style={{ width: 200 }} />
                </Form.Item>
              )}
            </Space>
          </Form.Item>
          <Form.Item name="data_scope" label="数据范围" extra="控制该角色可以访问哪些数据">
            <Select
              placeholder="选择数据访问范围"
              options={DATA_SCOPE_OPTIONS.map(o => ({
                label: (
                  <div>
                    <div>{o.label}</div>
                    <Text type="secondary" style={{ fontSize: 12 }}>{o.desc}</Text>
                  </div>
                ),
                value: o.value,
              }))}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Roles;
