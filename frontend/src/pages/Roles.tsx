import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, App, Tabs, Tag, Select, Dropdown } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SafetyCertificateOutlined, TeamOutlined, MoreOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Role } from '../api/role';
import { roleApi } from '../api/role';
import { mcpToolApi } from '../api/mcpTool';
import { skillApi } from '../api/skill';
import { useAuth } from '../contexts/AuthContext';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

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

const DATA_SCOPE_OPTIONS = [
  { label: '全部数据', value: 'all', desc: '可访问租户下所有数据' },
  { label: '本部门数据', value: 'department', desc: '仅可访问本部门数据' },
  { label: '仅个人数据', value: 'self', desc: '仅可访问自己创建的数据' },
];

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
  const isMobile = window.innerWidth < 768;

  // MCP工具和技能列表（用于权限选择）
  const [mcpTools, setMcpTools] = useState<{id: string; name: string; description?: string}[]>([]);
  const [skills, setSkills] = useState<{id: string; name: string; description?: string}[]>([]);

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

  const loadToolsAndSkills = async () => {
    if (!currentTenant) return;
    try {
      const [toolsRes, skillsRes] = await Promise.all([
        mcpToolApi.list(currentTenant).catch(() => ({ data: [] })),
        skillApi.list(currentTenant).catch(() => ({ data: [] })),
      ]);
      const toolsData = Array.isArray(toolsRes.data) ? toolsRes.data : [];
      const skillsData = Array.isArray(skillsRes.data) ? skillsRes.data : [];
      setMcpTools(toolsData.map((t: {id: string; name: string; description?: string}) => ({ id: t.id, name: t.name, description: t.description })));
      setSkills(skillsData.map((s: {id: string; name: string; description?: string}) => ({ id: s.id, name: s.name, description: s.description })));
    } catch {
      // 静默失败
    }
  };

  useEffect(() => { load(); }, [currentTenant]);
  useEffect(() => { loadToolsAndSkills(); }, [currentTenant]);

  const onOk = async () => {
    const values = await form.validateFields();
    if (values.tools && Array.isArray(values.tools)) {
      values.tools = JSON.stringify(values.tools);
    }
    if (values.allowed_mcp_tools && Array.isArray(values.allowed_mcp_tools)) {
      values.allowed_mcp_tools = JSON.stringify(values.allowed_mcp_tools);
    }
    if (values.allowed_skills && Array.isArray(values.allowed_skills)) {
      values.allowed_skills = JSON.stringify(values.allowed_skills);
    }
    try {
      if (editing) { await roleApi.update(currentTenant, editing.id, values); message.success('更新成功'); }
      else { await roleApi.create(currentTenant, values); message.success('创建成功'); }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '操作失败')
    }
  };

  const onDelete = async (id: string) => {
    try { await roleApi.delete(currentTenant, id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const openModal = (record?: Role) => {
    if (record) {
      setEditing(record);
      let toolsArray: string[] = [];
      if (record.tools) {
        try { toolsArray = JSON.parse(record.tools); } catch { toolsArray = []; }
      }
      let allowedMCPTools: string[] = [];
      if (record.allowed_mcp_tools) {
        try { allowedMCPTools = JSON.parse(record.allowed_mcp_tools); } catch { allowedMCPTools = []; }
      }
      let allowedSkills: string[] = [];
      if (record.allowed_skills) {
        try { allowedSkills = JSON.parse(record.allowed_skills); } catch { allowedSkills = []; }
      }
      form.setFieldsValue({ ...record, tools: toolsArray, allowed_mcp_tools: allowedMCPTools, allowed_skills: allowedSkills });
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
      <Space direction={isMobile ? 'vertical' : 'horizontal'} size={4}>
        <span>{v}</span>
        {r.is_default && <Tag color="blue">默认</Tag>}
      </Space>
    )},
    ...(!isMobile ? [
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
      { title: 'MCP工具权限', dataIndex: 'allowed_mcp_tools', key: 'allowed_mcp_tools', render: (v: string) => {
        if (!v) return <Text type="secondary">全部（未限制）</Text>;
        try {
          const ids: string[] = JSON.parse(v);
          if (ids.length === 0) return <Text type="secondary">无权限</Text>;
          return <Tag color="orange">{ids.length} 个工具</Tag>;
        } catch { return <Text type="secondary">-</Text>; }
      }},
      { title: '技能权限', dataIndex: 'allowed_skills', key: 'allowed_skills', render: (v: string) => {
        if (!v) return <Text type="secondary">全部（未限制）</Text>;
        try {
          const ids: string[] = JSON.parse(v);
          if (ids.length === 0) return <Text type="secondary">无权限</Text>;
          return <Tag color="purple">{ids.length} 个技能</Tag>;
        } catch { return <Text type="secondary">-</Text>; }
      }},
      { title: '限流', dataIndex: 'rate_limit', key: 'rate_limit', render: (v: string) => v || <Text type="secondary">不限</Text> },
      { title: '数据范围', dataIndex: 'data_scope', key: 'data_scope', render: (v: string) => {
        const opt = DATA_SCOPE_OPTIONS.find(o => o.value === v);
        return opt ? <Tag>{opt.label}</Tag> : (v || <Text type="secondary">-</Text>);
      }},
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 60 : 150, render: (_: unknown, record: Role) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => openModal(record) },
          ...(!record.is_default ? [{ key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDelete(record.id) }] : []),
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => openModal(record)}>编辑</Button>
          {!record.is_default && (
            <Popconfirm title="确认删除?" onConfirm={() => onDelete(record.id)}>
              <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </Space>
      )
    )},
  ];

  const systemColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
      { title: '工具权限', dataIndex: 'tools', key: 'tools', ellipsis: true },
      { title: '限流', dataIndex: 'rate_limit', key: 'rate_limit' },
      { title: '数据范围', dataIndex: 'data_scope', key: 'data_scope' },
    ] : []),
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
          <Table 
            dataSource={tenantRoles} 
            columns={tenantColumns} 
            rowKey="id" 
            loading={loading} 
            pagination={false}
            size={isMobile ? 'small' : 'middle'}
            scroll={isMobile ? { x: 300 } : undefined}
          />
        </>
      ),
    },
    ...(isAdmin ? [{
      key: 'system',
      label: <span><SafetyCertificateOutlined /> 系统角色</span>,
      children: (
        <Table 
          dataSource={systemRoles} 
          columns={systemColumns} 
          rowKey="id" 
          loading={loading} 
          pagination={false}
          size={isMobile ? 'small' : 'middle'}
          scroll={isMobile ? { x: 300 } : undefined}
        />
      ),
    }] : []),
  ];

  return (
    <div>
      <Title level={isMobile ? 4 : 3}>角色管理</Title>
      <Tabs items={tabItems} defaultActiveKey="tenant" size={isMobile ? 'small' : 'middle'} />
      <Modal 
        title={editing ? '编辑角色' : '新建角色'} 
        open={modalOpen} 
        onOk={onOk} 
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '90%' : 600}
      >
        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input placeholder="如：开发者、运维人员、访客" />
          </Form.Item>
          <Form.Item name="description" label="角色描述">
            <Input.TextArea rows={2} placeholder="描述该角色的职责和权限范围" />
          </Form.Item>
          <Form.Item name="tools" label="功能权限" extra="选择该角色可以使用的功能模块（UI菜单权限）">
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
          <Form.Item name="allowed_mcp_tools" label="MCP工具权限" extra="选择该角色可以使用的MCP工具。不选择表示无MCP工具权限，留空则无法使用任何MCP工具">
            <Select
              mode="multiple"
              placeholder="选择允许使用的MCP工具"
              options={mcpTools.map(t => ({
                label: (
                  <div>
                    <div>{t.name}</div>
                    {t.description && <Text type="secondary" style={{ fontSize: 12 }}>{t.description}</Text>}
                  </div>
                ),
                value: t.id,
              }))}
              maxTagCount="responsive"
              allowClear
              notFoundContent="暂无MCP工具"
            />
          </Form.Item>
          <Form.Item name="allowed_skills" label="技能权限" extra="选择该角色可以执行的技能。不选择表示无技能权限，留空则无法使用任何技能">
            <Select
              mode="multiple"
              placeholder="选择允许使用的技能"
              options={skills.map(s => ({
                label: (
                  <div>
                    <div>{s.name}</div>
                    {s.description && <Text type="secondary" style={{ fontSize: 12 }}>{s.description}</Text>}
                  </div>
                ),
                value: s.id,
              }))}
              maxTagCount="responsive"
              allowClear
              notFoundContent="暂无技能"
            />
          </Form.Item>
          <Form.Item label="访问频率限制">
            <Space direction={isMobile ? 'vertical' : 'horizontal'} style={{ width: '100%' }}>
              <Select
                value={rateLimitMode}
                onChange={(v) => {
                  setRateLimitMode(v);
                  if (v !== 'custom') {
                    form.setFieldValue('rate_limit', v);
                  }
                }}
                style={{ width: isMobile ? '100%' : 160 }}
                options={RATE_LIMIT_PRESETS}
              />
              {rateLimitMode === 'custom' && (
                <Form.Item name="rate_limit" noStyle>
                  <Input placeholder="如：200/hour, 50/minute" style={{ width: isMobile ? '100%' : 200 }} />
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
