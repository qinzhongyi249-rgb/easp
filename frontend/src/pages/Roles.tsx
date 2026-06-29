import React, { useMemo, useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, App, Tabs, Tag, Select, Dropdown, Alert, Card, Statistic, Steps, Divider } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SafetyCertificateOutlined, TeamOutlined, MoreOutlined, KeyOutlined, ToolOutlined, AppstoreOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { Role } from '../api/role';
import { roleApi } from '../api/role';
import { mcpToolApi } from '../api/mcpTool';
import { skillApi } from '../api/skill';
import { useAuth } from '../contexts/AuthContext';
import { FEATURE_MENU_PERMISSIONS } from '../config/menuPermissions';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

const TOOL_OPTIONS = FEATURE_MENU_PERMISSIONS;

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
  const [mcpTools, setMcpTools] = useState<{id: string; name: string; description?: string; status?: string; is_builtin?: boolean; locked?: boolean}[]>([]);
  const [skills, setSkills] = useState<{id: string; name: string; description?: string; status?: string}[]>([]);

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
        mcpToolApi.listUsable(currentTenant).catch(() => ({ data: [] })),
        skillApi.listUsable(currentTenant).catch(() => ({ data: [] })),
      ]);
      const toolsData = Array.isArray(toolsRes.data) ? toolsRes.data : [];
      const skillsData = Array.isArray(skillsRes.data) ? skillsRes.data : [];
      setMcpTools(toolsData.map((t: {id: string; name: string; description?: string; status?: string; is_builtin?: boolean; locked?: boolean}) => ({ id: t.id, name: t.name, description: t.description, status: t.status, is_builtin: t.is_builtin, locked: t.locked })));
      setSkills(skillsData.map((s: {id: string; name: string; description?: string; status?: string}) => ({ id: s.id, name: s.name, description: s.description, status: s.status })));
    } catch {
      // 静默失败
    }
  };

  useEffect(() => { load(); }, [currentTenant]);
  useEffect(() => { loadToolsAndSkills(); }, [currentTenant]);

  const lockedBuiltinMCPToolIds = useMemo(
    () => mcpTools.filter(t => t.is_builtin || t.locked).map(t => t.id),
    [mcpTools]
  );
  const isTenantAdminRole = (role?: Role | null) => Boolean(role && !role.is_system && role.name === '管理员');
  const isEditingTenantAdminRole = isTenantAdminRole(editing);
  const mergeLockedBuiltinMCPTools = (ids?: string[], forceAdmin = false) => {
    const merged = new Set([...(ids || []), ...((forceAdmin || isEditingTenantAdminRole) ? lockedBuiltinMCPToolIds : [])]);
    return Array.from(merged).filter(Boolean);
  };

  useEffect(() => {
    if (!modalOpen || !isEditingTenantAdminRole || lockedBuiltinMCPToolIds.length === 0) return;
    const current = form.getFieldValue('allowed_mcp_tools') || [];
    const merged = mergeLockedBuiltinMCPTools(current);
    if (merged.length !== current.length) {
      form.setFieldValue('allowed_mcp_tools', merged);
    }
  }, [modalOpen, isEditingTenantAdminRole, lockedBuiltinMCPToolIds, form]);

  const onOk = async () => {
    const values = await form.validateFields();
    if (values.tools && Array.isArray(values.tools)) {
      values.tools = JSON.stringify(values.tools);
    }
    if (isEditingTenantAdminRole || Array.isArray(values.allowed_mcp_tools)) {
      const selectedMCPTools = Array.isArray(values.allowed_mcp_tools) ? values.allowed_mcp_tools : [];
      values.allowed_mcp_tools = JSON.stringify(mergeLockedBuiltinMCPTools(selectedMCPTools));
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
        try { 
          const parsed = JSON.parse(record.allowed_mcp_tools);
          // 过滤掉已经被删除的MCP工具ID
          const existingIds = new Set(mcpTools.map(t => t.id));
          allowedMCPTools = Array.isArray(parsed) ? parsed.filter(id => existingIds.has(id)) : [];
        } catch { 
          allowedMCPTools = []; 
        }
      }
      let allowedSkills: string[] = [];
      if (record.allowed_skills) {
        try { 
          const parsed = JSON.parse(record.allowed_skills);
          // 过滤掉已经被删除的技能ID
          const existingIds = new Set(skills.map(s => s.id));
          allowedSkills = Array.isArray(parsed) ? parsed.filter(id => existingIds.has(id)) : [];
        } catch { 
          allowedSkills = []; 
        }
      }
      form.setFieldsValue({
        ...record,
        tools: toolsArray,
        allowed_mcp_tools: isTenantAdminRole(record) ? mergeLockedBuiltinMCPTools(allowedMCPTools, true) : allowedMCPTools,
        allowed_skills: allowedSkills,
      });
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

  const countJsonArray = (value?: string) => {
    if (!value) return 0;
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed.length : 0;
    } catch { return 0; }
  };

  const hasWildcardTools = (value?: string) => {
    if (!value) return false;
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) && parsed.includes('*');
    } catch { return false; }
  };

  const roleSummary = {
    tenant: tenantRoles.length,
    system: systemRoles.length,
    wildcard: tenantRoles.filter(r => hasWildcardTools(r.tools)).length,
    mcpGranted: tenantRoles.reduce((sum, r) => sum + countJsonArray(r.allowed_mcp_tools), 0),
    skillGranted: tenantRoles.reduce((sum, r) => sum + countJsonArray(r.allowed_skills), 0),
  };

  const renderRoleCard = (role: Role) => {
    const menuCount = hasWildcardTools(role.tools) ? '全部' : countJsonArray(role.tools);
    const mcpCount = countJsonArray(role.allowed_mcp_tools);
    const skillCount = countJsonArray(role.allowed_skills);
    return (
      <Card key={role.id} size="small" hoverable>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Space wrap>
            <Text strong>{role.name}</Text>
            {role.is_default && <Tag color="blue">默认</Tag>}
            {hasWildcardTools(role.tools) && <Tag color="gold">全功能</Tag>}
          </Space>
          {role.description && <Text type="secondary">{role.description}</Text>}
          <Space wrap size={[4, 4]}>
            <Tag icon={<AppstoreOutlined />} color="blue">菜单 {menuCount}</Tag>
            <Tag icon={<ToolOutlined />} color="orange">MCP {mcpCount}</Tag>
            <Tag icon={<KeyOutlined />} color="purple">Skill {skillCount}</Tag>
            <Tag>{DATA_SCOPE_OPTIONS.find(o => o.value === role.data_scope)?.label || role.data_scope || '数据未配置'}</Tag>
          </Space>
          <Space wrap>
            <Button size="small" icon={<EditOutlined />} onClick={() => openModal(role)}>配置授权</Button>
            {!role.is_default && (
              <Popconfirm title="确认删除?" onConfirm={() => onDelete(role.id)}>
                <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
              </Popconfirm>
            )}
          </Space>
        </Space>
      </Card>
    );
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
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', flexDirection: isMobile ? 'column' : 'row', gap: 12 }}>
            <Text type="secondary">租户角色负责菜单可见、MCP 工具、Skill、数据范围和限流授权。</Text>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()}>新建角色</Button>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
            {tenantRoles.map(renderRoleCard)}
          </div>
          <Card title="角色明细" extra={<Text type="secondary">用于查看完整授权字段</Text>}>
            <Table
              dataSource={tenantRoles}
              columns={tenantColumns}
              rowKey="id"
              loading={loading}
              pagination={false}
              size={isMobile ? 'small' : 'middle'}
              scroll={isMobile ? { x: 300 } : undefined}
            />
          </Card>
        </Space>
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
      <div style={{ marginBottom: 16 }}>
        <Title level={isMobile ? 4 : 3} style={{ marginBottom: 4 }}><SafetyCertificateOutlined /> 授权工作台</Title>
        <Text type="secondary">统一配置角色的菜单权限、MCP 工具权限、Skill 权限、数据范围和限流。工具/Skill 获取与执行链路都会按这里的授权判断。</Text>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="授权闭环：先导入并发布工具/Skill，再在角色中授权，最后由 AI 助手按权限暴露和执行。"
          description="前端菜单只负责展示入口；真正的 MCP/Skill 可用性由后端根据用户角色实时过滤。"
        />
        <Card>
          <Steps
            size="small"
            direction={isMobile ? 'vertical' : 'horizontal'}
            items={[
              { title: '选择角色', description: '租户角色或系统角色' },
              { title: '菜单入口', description: '控制后台功能可见' },
              { title: '工具与 Skill', description: '控制助手可调用能力' },
              { title: '数据与限流', description: '控制访问边界' },
            ]}
          />
        </Card>
        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(5, minmax(0, 1fr))', gap: 12 }}>
          <Card><Statistic title="租户角色" value={roleSummary.tenant} prefix={<TeamOutlined />} /></Card>
          <Card><Statistic title="系统角色" value={roleSummary.system} prefix={<SafetyCertificateOutlined />} /></Card>
          <Card><Statistic title="全功能角色" value={roleSummary.wildcard} prefix={<CheckCircleOutlined />} /></Card>
          <Card><Statistic title="MCP授权项" value={roleSummary.mcpGranted} prefix={<ToolOutlined />} /></Card>
          <Card><Statistic title="Skill授权项" value={roleSummary.skillGranted} prefix={<KeyOutlined />} /></Card>
        </div>
        <Tabs items={tabItems} defaultActiveKey="tenant" size={isMobile ? 'small' : 'middle'} />
      </Space>
      <Modal
        title={editing ? `配置授权 — ${editing.name}` : '新建授权角色'}
        open={modalOpen}
        onOk={onOk}
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '92%' : 780}
      >
        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="这里同时配置后台入口和助手能力授权。"
            description="菜单权限影响前端入口可见；MCP/Skill 权限影响 AI 助手获取、暴露和执行工具的后端判断。"
          />
          <Divider>角色基础信息</Divider>
          <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input placeholder="如：开发者、运维人员、访客" />
          </Form.Item>
          <Form.Item name="description" label="角色描述">
            <Input.TextArea rows={2} placeholder="描述该角色的职责和权限范围" />
          </Form.Item>
          <Divider>后台菜单入口</Divider>
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
                title: o.label,
                value: o.value,
                searchText: `${o.label} ${o.desc} ${o.value}`,
              }))}
              optionLabelProp="title"
              filterOption={(input, option) =>
                String(option?.searchText || option?.value || '')
                  .toLowerCase()
                  .includes(input.toLowerCase())
              }
              maxTagCount="responsive"
              allowClear
            />
          </Form.Item>
          <Divider>助手可调用能力</Divider>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message="只选择生产可授权资源。"
            description="草稿、测试中、停用的工具/Skill 不应出现在生产授权列表；管理员角色会自动保留内置锁定工具。"
          />
          <Form.Item name="allowed_mcp_tools" label="MCP工具权限" extra="仅可选择已启用且已发布的MCP工具；测试中/草稿/停用不会出现在这里，也不会被AI助手生产调用。">
            <Select
              mode="multiple"
              placeholder="选择允许使用的MCP工具"
              options={mcpTools.map(t => {
                const isLockedBuiltin = Boolean(t.is_builtin || t.locked);
                return {
                  label: (
                    <div>
                      <div>
                        {t.name}
                        {isLockedBuiltin && <Tag color="gold" style={{ marginLeft: 8 }}>内置锁定</Tag>}
                      </div>
                      {t.description && <Text type="secondary" style={{ fontSize: 12 }}>{t.description}</Text>}
                    </div>
                  ),
                  value: t.id,
                  disabled: isEditingTenantAdminRole && isLockedBuiltin,
                };
              })}
              onChange={(selected: string[]) => {
                if (isEditingTenantAdminRole) {
                  form.setFieldValue('allowed_mcp_tools', mergeLockedBuiltinMCPTools(selected));
                }
              }}
              maxTagCount="responsive"
              allowClear={!isEditingTenantAdminRole}
              notFoundContent="暂无MCP工具"
            />
          </Form.Item>
          <Form.Item name="allowed_skills" label="技能权限" extra="仅可选择已发布的技能；测试中/草稿/停用不会出现在这里，也不会被AI助手生产调用。">
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
          <Divider>数据边界与调用频率</Divider>
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
