import React, { useState, useEffect, useCallback } from 'react';
import {
  Alert,
  App,
  Badge,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Dropdown,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Segmented,
  Space,
  Statistic,
  Steps,
  Table,
  Tabs,
  Tag,
  Typography,
} from 'antd';
import {
  ApiOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
  CodeOutlined,
  CopyOutlined,
  DeleteOutlined,
  ImportOutlined,
  KeyOutlined,
  LockOutlined,
  MoreOutlined,
  PlusOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  UndoOutlined,
  UserOutlined,
  WarningOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { EmbedAppDiagnoseResult, EmbedAppGuide, ExternalUserBinding, TenantEmbedApp, TenantUser, UserIdentityBinding } from '../api/user';
import type { Role } from '../api/role';
import { userApi } from '../api/user';
import { roleApi } from '../api/role';
import AccessManualModal from '../components/AccessManualModal';

const { Title, Text, Paragraph } = Typography;
interface LayoutContext { currentTenant: string; }
type UserFilter = 'active' | 'deleted' | 'all';

type UserRow = TenantUser & { is_admin?: boolean; role_names?: string[] };

const parseMaybeJson = (value?: string | null) => {
  if (!value) return '-';
  try { return JSON.stringify(JSON.parse(value), null, 2); } catch { return value; }
};

const splitLines = (value?: string) => (value || '').split('\n').map(v => v.trim()).filter(Boolean);

const fmtTime = (value?: string | null) => value ? new Date(value).toLocaleString() : '-';
const daysSince = (value?: string | null) => value ? Math.floor((Date.now() - new Date(value).getTime()) / 86400000) : null;
const userRisk = (user: UserRow): { label: string; color: string; reason: string } => {
  if (user.deleted_at) return { label: '已注销', color: 'default', reason: '用户已软删除，可按需恢复。' };
  if (user.status !== 'active') return { label: user.status || '异常状态', color: 'red', reason: '用户状态非 active，需确认是否允许登录。' };
  if (!user.role_names || user.role_names.length === 0) return { label: '未授权', color: 'orange', reason: '用户没有角色，通常无法获得菜单/工具/Skill 权限。' };
  const lastLoginDays = daysSince(user.last_login_at);
  if (lastLoginDays === null && (user.login_count || 0) === 0) return { label: '未登录', color: 'orange', reason: '用户创建后尚未登录，建议确认账号是否已交付。' };
  if (lastLoginDays !== null && lastLoginDays > 90) return { label: '长期未登录', color: 'orange', reason: '超过 90 天未登录，建议确认是否仍需保留。' };
  return { label: '正常', color: 'green', reason: '用户状态、角色和登录情况正常。' };
};

const parseJsonList = (value?: string | null): string[] => {
  if (!value) return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed.map(String) : [];
  } catch {
    return value.split(/[\n,]/).map(v => v.trim()).filter(Boolean);
  }
};

const StepCodeBlock: React.FC<{ title: string; code: string; note?: string }> = ({ title, code, note }) => (
  <Card size="small" title={title} extra={<Text copyable={{ text: code }}>复制</Text>}>
    {note && <Text type="secondary" style={{ display: 'block', marginBottom: 8 }}>{note}</Text>}
    <pre style={{ margin: 0, padding: 12, borderRadius: 8, background: '#0f172a', color: '#e2e8f0', overflow: 'auto', fontSize: 12, lineHeight: 1.6 }}>{code}</pre>
  </Card>
);

const Users: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message, modal } = App.useApp();
  const [activeTab, setActiveTab] = useState('users');
  const [users, setUsers] = useState<UserRow[]>([]);
  const [externalUsers, setExternalUsers] = useState<ExternalUserBinding[]>([]);
  const [identities, setIdentities] = useState<UserIdentityBinding[]>([]);
  const [embedApps, setEmbedApps] = useState<TenantEmbedApp[]>([]);
  const [tenantRoles, setTenantRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [externalLoading, setExternalLoading] = useState(false);
  const [identityLoading, setIdentityLoading] = useState(false);
  const [appLoading, setAppLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState<{ user?: TenantUser; identities: UserIdentityBinding[] }>({ identities: [] });
  const [appModalOpen, setAppModalOpen] = useState(false);
  const [selectedApp, setSelectedApp] = useState<TenantEmbedApp | null>(null);
  const [guideOpen, setGuideOpen] = useState(false);
  const [guideLoading, setGuideLoading] = useState(false);
  const [guide, setGuide] = useState<EmbedAppGuide | null>(null);
  const [diagnoseOpen, setDiagnoseOpen] = useState(false);
  const [diagnoseLoading, setDiagnoseLoading] = useState(false);
  const [diagnoseResult, setDiagnoseResult] = useState<EmbedAppDiagnoseResult | null>(null);
  const [importModalOpen, setImportModalOpen] = useState(false);
  const [manualOpen, setManualOpen] = useState(false);
  const [form] = Form.useForm();
  const [appForm] = Form.useForm();
  const [diagnoseForm] = Form.useForm();
  const [importForm] = Form.useForm();
  const [filter, setFilter] = useState<UserFilter>('active');
  const [userKeyword, setUserKeyword] = useState('');
  const [externalKeyword, setExternalKeyword] = useState('');
  const [externalSystem, setExternalSystem] = useState('');
  const [identityKeyword, setIdentityKeyword] = useState('');
  const [identityProvider, setIdentityProvider] = useState('');
  const isMobile = window.innerWidth < 768;

  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const [roleModalUser, setRoleModalUser] = useState<UserRow | null>(null);
  const [userRoleIds, setUserRoleIds] = useState<string[]>([]);
  const [roleSaving, setRoleSaving] = useState(false);

  const [resetModalOpen, setResetModalOpen] = useState(false);
  const [resetUser, setResetUser] = useState<UserRow | null>(null);
  const [resetPassword, setResetPassword] = useState('');
  const [resetLoading, setResetLoading] = useState(false);
  const [resetSaved, setResetSaved] = useState(false);

  const loadUsers = useCallback(async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const [u, r] = await Promise.all([
        userApi.listByTenant(currentTenant, { keyword: userKeyword || undefined, limit: 300 }),
        roleApi.list(currentTenant),
      ]);
      setUsers(u.data || []);
      const roleData = r.data as unknown as { tenant_roles?: Role[]; system_roles?: Role[] };
      // 用户分配角色只允许使用当前租户角色；系统级角色只能由后端专门授权，不能混入下拉框。
      setTenantRoles(roleData.tenant_roles || []);
    } catch { message.error('加载用户失败'); }
    finally { setLoading(false); }
  }, [currentTenant, userKeyword]);

  const loadExternalUsers = useCallback(async () => {
    if (!currentTenant) return;
    setExternalLoading(true);
    try {
      const res = await userApi.listExternalUsers(currentTenant, {
        external_system: externalSystem || undefined,
        keyword: externalKeyword || undefined,
        limit: 300,
      });
      setExternalUsers(res.data || []);
    } catch { message.error('加载外部用户失败'); }
    finally { setExternalLoading(false); }
  }, [currentTenant, externalSystem, externalKeyword]);

  const loadIdentities = useCallback(async () => {
    if (!currentTenant) return;
    setIdentityLoading(true);
    try {
      const res = await userApi.listUserIdentities(currentTenant, {
        provider: identityProvider || undefined,
        keyword: identityKeyword || undefined,
        limit: 300,
      });
      setIdentities(res.data || []);
    } catch { message.error('加载第三方身份失败'); }
    finally { setIdentityLoading(false); }
  }, [currentTenant, identityProvider, identityKeyword]);

  const loadEmbedApps = useCallback(async () => {
    if (!currentTenant) return;
    setAppLoading(true);
    try {
      const res = await userApi.listEmbedApps(currentTenant);
      setEmbedApps(res.data || []);
    } catch { message.error('加载接入应用失败'); }
    finally { setAppLoading(false); }
  }, [currentTenant]);

  useEffect(() => {
    loadUsers();
    loadExternalUsers();
    loadIdentities();
    loadEmbedApps();
  }, [currentTenant]);

  const onCreate = async () => {
    const values = await form.validateFields();
    try { await userApi.create(currentTenant, values); message.success('创建成功'); setModalOpen(false); form.resetFields(); loadUsers(); }
    catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '创建失败'); }
  };

  const onDelete = async (id: string) => {
    try { await userApi.delete(currentTenant, id); message.success('已删除'); loadUsers(); }
    catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '删除失败'); }
  };

  const onRestore = async (id: string) => {
    try { await userApi.restore(currentTenant, id); message.success('已恢复'); loadUsers(); }
    catch { message.error('恢复失败'); }
  };

  const showDetail = async (record: UserRow) => {
    setDetailOpen(true);
    setDetailLoading(true);
    try {
      const res = await userApi.get(currentTenant, record.id);
      setDetail({ user: res.data.user, identities: res.data.identities || [] });
    } catch { message.error('加载用户详情失败'); }
    finally { setDetailLoading(false); }
  };

  const onResetPassword = useCallback(async (user: UserRow) => {
    setResetUser(user); setResetPassword(''); setResetSaved(false); setResetModalOpen(true); setResetLoading(true);
    try { const res = await userApi.generateResetPassword(currentTenant, user.id); setResetPassword(res.data.password); }
    catch { message.error('生成密码失败'); setResetModalOpen(false); }
    finally { setResetLoading(false); }
  }, [currentTenant]);

  const confirmResetPassword = async () => {
    if (!resetUser || !resetPassword) return;
    setResetLoading(true);
    try { await userApi.confirmResetPassword(currentTenant, resetUser.id, resetPassword); setResetSaved(true); message.success('密码已重置'); }
    catch { message.error('保存密码失败'); }
    finally { setResetLoading(false); }
  };

  const copyText = async (text: string, ok = '已复制') => {
    const value = text || '';
    if (!value) {
      message.warning('没有可复制的内容');
      return;
    }

    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(value);
        message.success(ok);
        return;
      }
    } catch {
      // HTTP/IP 访问、微信内置浏览器或移动端 WebView 可能拒绝 Clipboard API，继续走 fallback。
    }

    const textarea = document.createElement('textarea');
    textarea.value = value;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.left = '-9999px';
    textarea.style.top = '0';
    textarea.style.opacity = '0';
    document.body.appendChild(textarea);
    textarea.focus();
    textarea.select();
    textarea.setSelectionRange(0, textarea.value.length);

    try {
      const copied = document.execCommand('copy');
      if (copied) {
        message.success(ok);
      } else {
        message.error('复制失败，请长按密码手动复制');
      }
    } catch {
      message.error('复制失败，请长按密码手动复制');
    } finally {
      document.body.removeChild(textarea);
    }
  };

  const openRoleModal = useCallback(async (user: UserRow) => {
    setRoleModalUser(user); setRoleModalOpen(true);
    try {
      const res = await userApi.getRoles(user.id);
      const roles = (res.data || []) as Role[];
      setUserRoleIds(roles.map(r => r.id));
    } catch { setUserRoleIds([]); }
  }, []);

  const saveRoles = async () => {
    if (!roleModalUser) return;
    setRoleSaving(true);
    try {
      const res = await userApi.getRoles(roleModalUser.id);
      const currentRoles = (res.data || []) as Role[];
      const currentIds = currentRoles.map(r => r.id);
      const toAdd = userRoleIds.filter(id => !currentIds.includes(id));
      const toRemove = currentIds.filter(id => !userRoleIds.includes(id));
      await Promise.all([
        ...toAdd.map(roleId => userApi.assignRole(roleModalUser.id, roleId)),
        ...toRemove.map(roleId => userApi.revokeRole(roleModalUser.id, roleId)),
      ]);
      message.success('角色更新成功'); setRoleModalOpen(false); loadUsers();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '角色更新失败');
    } finally { setRoleSaving(false); }
  };

  const onCreateEmbedApp = async () => {
    const values = await appForm.validateFields();
    const payload = {
      ...values,
      allowed_origins: splitLines(values.allowed_origins),
      allowed_scopes: splitLines(values.allowed_scopes),
    };
    try {
      const res = await userApi.createEmbedApp(currentTenant, payload);
      message.success('接入应用已创建');
      setAppModalOpen(false); appForm.resetFields(); loadEmbedApps();
      modal.info({
        title: '请立即保存 App Secret',
        width: 620,
        content: <div>
          <Alert type="warning" showIcon message="App Secret 只显示一次，关闭后无法再次查看。" style={{ marginBottom: 12 }} />
          <Descriptions column={1} size="small" bordered>
            <Descriptions.Item label="App ID"><Text copyable>{res.data.app.app_id}</Text></Descriptions.Item>
            <Descriptions.Item label="App Secret"><Text code copyable>{res.data.app_secret}</Text></Descriptions.Item>
          </Descriptions>
        </div>,
      });
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '创建接入应用失败');
    }
  };

  const openGuide = async (app: TenantEmbedApp) => {
    setSelectedApp(app);
    setGuideOpen(true);
    setGuide(null);
    setGuideLoading(true);
    try {
      const res = await userApi.getEmbedAppGuide(currentTenant, app.id);
      setGuide(res.data);
    } catch { message.error('加载接入指南失败'); }
    finally { setGuideLoading(false); }
  };

  const openDiagnose = (app: TenantEmbedApp) => {
    setSelectedApp(app);
    setDiagnoseResult(null);
    diagnoseForm.resetFields();
    diagnoseForm.setFieldsValue({ origin: (parseMaybeJson(app.allowed_origins) || '').split('\n')[0]?.replace(/[\[\]",]/g, '').trim(), external_user_id: '' });
    setDiagnoseOpen(true);
  };

  const runDiagnose = async () => {
    if (!selectedApp) return;
    const values = await diagnoseForm.validateFields();
    setDiagnoseLoading(true);
    try {
      const res = await userApi.diagnoseEmbedApp(currentTenant, selectedApp.id, values);
      setDiagnoseResult(res.data);
    } catch { message.error('联调检测失败'); }
    finally { setDiagnoseLoading(false); }
  };

  const gotoExternalUsers = (app?: TenantEmbedApp) => {
    if (app?.external_system) setExternalSystem(app.external_system);
    setActiveTab('external');
  };

  const gotoAuditLogs = (app?: TenantEmbedApp) => {
    const params = new URLSearchParams();
    if (app?.app_id) params.set('source_app_id', app.app_id);
    window.location.href = `/audit-logs${params.toString() ? `?${params.toString()}` : ''}`;
  };

  const onImportExternalUser = async () => {
    const values = await importForm.validateFields();
    let profile: Record<string, unknown> | undefined;
    let attributes: Record<string, unknown> | undefined;
    try { profile = values.profile ? JSON.parse(values.profile) : undefined; } catch { message.error('profile 不是合法 JSON'); return; }
    try { attributes = values.attributes ? JSON.parse(values.attributes) : undefined; } catch { message.error('attributes 不是合法 JSON'); return; }
    const identities = [
      values.wechat_id ? { provider: 'wechat', provider_user_id: values.wechat_id, display_name: values.wechat_name || values.display_name } : null,
      values.feishu_id ? { provider: 'feishu', provider_user_id: values.feishu_id, display_name: values.feishu_name || values.display_name } : null,
    ].filter(Boolean) as Array<{ provider: string; provider_user_id: string; display_name?: string }>;
    try {
      const res = await userApi.importExternalUsers(currentTenant, {
        external_system: values.external_system,
        default_password: values.default_password,
        users: [{
          account: values.account,
          external_user_id: values.external_user_id,
          password: values.password,
          user_uid: values.user_uid,
          display_name: values.display_name,
          email: values.email,
          phone: values.phone,
          avatar: values.avatar,
          department: values.department,
          position: values.position,
          profile,
          attributes,
          identities,
        }],
      });
      const item = res.data.items?.[0];
      if (item?.status === 'imported' || item?.status === 'created') {
        message.success(`外部用户导入成功${item.login_identifier ? `，登录账号：${item.login_identifier}` : ''}${item.password_configured ? '，密码已配置' : '，未配置可登录密码'}`);
      } else message.warning(item?.error || item?.status || '导入完成但状态未知');
      setImportModalOpen(false); importForm.resetFields(); loadExternalUsers(); loadIdentities(); loadUsers();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '导入失败');
    }
  };

  const filteredUsers = users.filter(user => {
    if (filter === 'active') return !user.deleted_at;
    if (filter === 'deleted') return !!user.deleted_at;
    return true;
  });

  const activeUsers = users.filter(user => !user.deleted_at && user.status === 'active');
  const deletedUsers = users.filter(user => !!user.deleted_at);
  const adminUsers = users.filter(user => user.is_admin || user.role_names?.some(name => name.includes('管理员')));
  const riskyUsers = users.filter(user => ['未授权', '未登录', '长期未登录', '异常状态'].includes(userRisk(user).label));
  const identityLinkedUsers = new Set(identities.map(item => item.user_id).filter(Boolean)).size;
  const externalLinkedUsers = new Set(externalUsers.map(item => item.user_id).filter(Boolean)).size;

  const getActionItems = (record: UserRow) => {
    if (record.deleted_at) return [{ key: 'restore', label: '恢复', icon: <UndoOutlined />, onClick: () => onRestore(record.id) }];
    if (record.is_admin) return [{ key: 'detail', label: '详情', onClick: () => showDetail(record) }];
    return [
      { key: 'detail', label: '详情', onClick: () => showDetail(record) },
      { key: 'role', label: '角色管理', icon: <SafetyCertificateOutlined />, onClick: () => openRoleModal(record) },
      { key: 'reset', label: '重置密码', icon: <KeyOutlined />, onClick: () => onResetPassword(record) },
      { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDelete(record.id) },
    ];
  };

  const userColumns = [
    {
      title: '用户', key: 'user', ellipsis: isMobile,
      render: (_: unknown, record: UserRow) => (
        <Space direction="vertical" size={2}>
          <Space size={4} wrap>
            <Text strong style={record.deleted_at ? { textDecoration: 'line-through', color: '#999' } : {}}>{record.display_name || record.account || record.email || record.phone}</Text>
            {record.is_admin && <Tag icon={<LockOutlined />} color="gold">超级管理员</Tag>}
            {!record.is_admin && record.role_names?.includes('管理员') && <Tag icon={<SafetyCertificateOutlined />} color="blue">管理员</Tag>}
            {record.deleted_at && <Tag color="default">已注销</Tag>}
            <Tag color={userRisk(record).color}>{userRisk(record).label}</Tag>
          </Space>
          <Text type="secondary" style={{ fontSize: 12 }}>账号: {record.account || '-'}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>邮箱/手机号: {record.email || '-'} / {record.phone || '-'}</Text>
          <Text type="secondary" style={{ fontSize: 12 }}>UID: {record.user_uid || '-'}</Text>
        </Space>
      )
    },
    ...(!isMobile ? [
      { title: '角色', dataIndex: 'role_names', key: 'role_names', render: (v: string[]) => <Space wrap>{(v || []).length ? (v || []).map(r => <Tag key={r}>{r}</Tag>) : <Tag color="orange">未授权</Tag>}</Space> },
      { title: '状态/风险', dataIndex: 'status', key: 'status', render: (_: string, record: UserRow) => <Space direction="vertical" size={2}><Tag color={record.status === 'active' ? 'green' : 'red'}>{record.status}</Tag><Tag color={userRisk(record).color}>{userRisk(record).label}</Tag></Space> },
      { title: '登录', key: 'login', render: (_: unknown, record: UserRow) => <Space direction="vertical" size={2}><Text>{record.login_count || 0} 次</Text><Text type="secondary" style={{ fontSize: 12 }}>{fmtTime(record.last_login_at)}</Text></Space> },
      { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: fmtTime },
    ] : []),
    {
      title: '操作', key: 'action', width: isMobile ? 64 : 260,
      render: (_: unknown, record: UserRow) => isMobile ? (
        <Dropdown menu={{ items: getActionItems(record) }} trigger={['click']}><Button type="text" icon={<MoreOutlined />} /></Dropdown>
      ) : (
        <Space wrap>
          <Button size="small" onClick={() => showDetail(record)}>详情</Button>
          {record.deleted_at ? <Popconfirm title="确认恢复此用户?" onConfirm={() => onRestore(record.id)}><Button size="small" type="primary" icon={<UndoOutlined />}>恢复</Button></Popconfirm> : record.is_admin ? <Tag color="default">不可操作</Tag> : <>
            <Button size="small" icon={<SafetyCertificateOutlined />} onClick={() => openRoleModal(record)}>角色</Button>
            <Button size="small" icon={<KeyOutlined />} onClick={() => onResetPassword(record)}>重置密码</Button>
            <Popconfirm title="确认删除此用户?" onConfirm={() => onDelete(record.id)}><Button size="small" danger icon={<DeleteOutlined />}>删除</Button></Popconfirm>
          </>}
        </Space>
      )
    },
  ];

  const externalColumns = [
    { title: '外部系统', dataIndex: 'external_system', key: 'external_system', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '外部用户ID', dataIndex: 'external_user_id', key: 'external_user_id', ellipsis: true },
    { title: '显示名', dataIndex: 'display_name', key: 'display_name' },
    { title: '邮箱/手机号', key: 'contact', render: (_: unknown, r: ExternalUserBinding) => <Text>{r.email || '-'} / {r.phone || '-'}</Text> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag> },
    { title: '绑定用户', dataIndex: 'user_id', key: 'user_id', ellipsis: true, render: (v: string) => <Text code>{v}</Text> },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
  ];

  const identityColumns = [
    { title: '来源', dataIndex: 'provider', key: 'provider', render: (v: string) => <Tag color={v === 'wechat' ? 'green' : v === 'feishu' ? 'purple' : 'blue'}>{v}</Tag> },
    { title: '第三方用户ID', dataIndex: 'provider_user_id', key: 'provider_user_id', ellipsis: true },
    { title: 'Union/Open ID', key: 'openid', ellipsis: true, render: (_: unknown, r: UserIdentityBinding) => <Text>{r.union_id || r.open_id || '-'}</Text> },
    { title: '显示名', dataIndex: 'display_name', key: 'display_name' },
    { title: '邮箱/手机号', key: 'contact', render: (_: unknown, r: UserIdentityBinding) => <Text>{r.email || '-'} / {r.phone || '-'}</Text> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag> },
    { title: 'EASP用户', dataIndex: 'user_id', key: 'user_id', ellipsis: true, render: (v: string) => <Text code>{v}</Text> },
  ];

  const appSummary = {
    total: embedApps.length,
    active: embedApps.filter(app => app.status === 'active').length,
    systems: new Set(embedApps.map(app => app.external_system).filter(Boolean)).size,
  };

  const renderAppCard = (app: TenantEmbedApp) => {
    const origins = parseJsonList(app.allowed_origins);
    return <Card key={app.id} size="small" hoverable style={{ height: '100%' }}>
      <Space direction="vertical" size="small" style={{ width: '100%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8, alignItems: 'flex-start' }}>
          <div>
            <Text strong style={{ fontSize: 16 }}>{app.name}</Text>
            <div><Text type="secondary">{app.external_system}</Text></div>
          </div>
          <Tag color={app.status === 'active' ? 'green' : 'default'}>{app.status}</Tag>
        </div>
        <Text copyable code style={{ maxWidth: '100%' }}>{app.app_id}</Text>
        <div style={{ minHeight: 44 }}>
          {origins.length ? origins.slice(0, 2).map(origin => <Tag key={origin} color="blue" style={{ marginBottom: 4 }}>{origin}</Tag>) : <Tag>来源不限</Tag>}
          {origins.length > 2 && <Tag>+{origins.length - 2}</Tag>}
        </div>
        <Text type="secondary">Token TTL：{app.token_ttl_seconds || 7200}s</Text>
        <Space wrap>
          <Button size="small" type="primary" icon={<CodeOutlined />} onClick={() => openGuide(app)}>接入指南</Button>
          <Button size="small" icon={<CheckCircleOutlined />} onClick={() => openDiagnose(app)}>联调检测</Button>
          <Button size="small" onClick={() => gotoExternalUsers(app)}>外部用户</Button>
          <Button size="small" onClick={() => gotoAuditLogs(app)}>审计</Button>
        </Space>
      </Space>
    </Card>;
  };

  const appColumns = [
    { title: '应用名称', dataIndex: 'name', key: 'name', render: (v: string, r: TenantEmbedApp) => <Space direction="vertical" size={2}><Text strong>{v}</Text><Text type="secondary" style={{ fontSize: 12 }}>业务系统后端换 Token，前端只用短期 token 嵌入助手</Text><Text type="secondary" style={{ fontSize: 12 }}>外部系统：{r.external_system}</Text></Space> },
    { title: 'App ID', dataIndex: 'app_id', key: 'app_id', ellipsis: true, render: (v: string) => <Text copyable code>{v}</Text> },
    { title: '来源白名单', dataIndex: 'allowed_origins', key: 'allowed_origins', ellipsis: true, render: (v?: string) => <Text style={{ whiteSpace: 'pre-wrap' }}>{parseMaybeJson(v)}</Text> },
    { title: 'Token TTL', dataIndex: 'token_ttl_seconds', key: 'token_ttl_seconds', render: (v: number) => `${v || 7200}s` },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'default'}>{v}</Tag> },
    {
      title: '接入操作', key: 'action', width: 340,
      render: (_: unknown, record: TenantEmbedApp) => <Space wrap>
        <Button size="small" type="primary" icon={<CodeOutlined />} onClick={() => openGuide(record)}>接入指南</Button>
        <Button size="small" icon={<CheckCircleOutlined />} onClick={() => openDiagnose(record)}>联调检测</Button>
        <Button size="small" onClick={() => gotoExternalUsers(record)}>外部用户</Button>
        <Button size="small" onClick={() => gotoAuditLogs(record)}>审计日志</Button>
      </Space>
    },
  ];

  const usersToolbar = <Space wrap style={{ marginBottom: 16 }}>
    <Segmented options={[{ label: '正常', value: 'active' }, { label: '已注销', value: 'deleted' }, { label: '全部', value: 'all' }]} value={filter} onChange={(v) => setFilter(v as UserFilter)} size={isMobile ? 'small' : 'middle'} />
    <Input.Search placeholder="搜索 UID/邮箱/手机号/显示名" allowClear value={userKeyword} onChange={e => setUserKeyword(e.target.value)} onSearch={loadUsers} style={{ width: isMobile ? '100%' : 260 }} />
    <Button icon={<ReloadOutlined />} onClick={loadUsers}>刷新</Button>
    <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setModalOpen(true); }}>新建用户</Button>
  </Space>;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: 12 }}>
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>租户用户运营工作台</Title>
          <Text type="secondary">统一管理平台用户、外部用户、第三方身份关联和嵌入式 AI 助手接入应用；权限主体仍是 EASP 内部用户。</Text>
        </div>
        <Button icon={<ApiOutlined />} onClick={() => setManualOpen(true)}>用户接入手册</Button>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="租户用户、外部身份和应用接入统一治理"
          description="用户删除采用软删除，可恢复；角色授权只使用租户角色，系统级角色不会混入分配下拉。外部用户和第三方身份用于映射与审计，真正权限主体仍是 EASP 内部用户。"
        />
        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr 1fr' : 'repeat(4, minmax(0, 1fr))', gap: 12 }}>
          <Card size="small"><Statistic title="租户用户" value={users.length} prefix={<UserOutlined />} /></Card>
          <Card size="small"><Statistic title="活跃用户" value={activeUsers.length} valueStyle={{ color: '#52c41a' }} prefix={<CheckCircleOutlined />} /></Card>
          <Card size="small"><Statistic title="已注销" value={deletedUsers.length} valueStyle={{ color: deletedUsers.length ? '#faad14' : undefined }} prefix={<UndoOutlined />} /></Card>
          <Card size="small"><Statistic title="需关注" value={riskyUsers.length} valueStyle={{ color: riskyUsers.length ? '#faad14' : '#52c41a' }} prefix={<WarningOutlined />} /></Card>
        </div>
        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
          <Card size="small" title="权限与角色">
            <Space direction="vertical" size={4}>
              <Text>管理员用户：<Text strong>{adminUsers.length}</Text></Text>
              <Text>租户角色：<Text strong>{tenantRoles.length}</Text></Text>
              <Text type="secondary">角色授权决定菜单、工具和 Skill 的最终可用能力。</Text>
            </Space>
          </Card>
          <Card size="small" title="身份映射">
            <Space direction="vertical" size={4}>
              <Text>外部用户：<Text strong>{externalUsers.length}</Text>，已绑定内部用户：<Text strong>{externalLinkedUsers}</Text></Text>
              <Text>第三方身份：<Text strong>{identities.length}</Text>，已绑定内部用户：<Text strong>{identityLinkedUsers}</Text></Text>
              <Text type="secondary">外部身份用于审计快照，不替代内部权限主体。</Text>
            </Space>
          </Card>
          <Card size="small" title="账号生命周期">
            <Space direction="vertical" size={4}>
              <Text><ClockCircleOutlined /> 从未/长期未登录：<Text strong>{riskyUsers.filter(u => ['未登录', '长期未登录'].includes(userRisk(u).label)).length}</Text></Text>
              <Text><SafetyCertificateOutlined /> 未授权：<Text strong>{riskyUsers.filter(u => userRisk(u).label === '未授权').length}</Text></Text>
              <Text type="secondary">删除为软删除，恢复入口在“已注销”筛选下可见。</Text>
            </Space>
          </Card>
        </div>
      </Space>

      <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        { key: 'users', label: '平台用户', children: <Card>{usersToolbar}<Table dataSource={filteredUsers} columns={userColumns} rowKey="id" loading={loading} size={isMobile ? 'small' : 'middle'} scroll={isMobile ? { x: 760 } : undefined} pagination={{ pageSize: 10 }} /></Card> },
        { key: 'external', label: '外部用户', children: <Card>
          <Space wrap style={{ marginBottom: 16 }}>
            <Input placeholder="外部系统，如 crm" allowClear value={externalSystem} onChange={e => setExternalSystem(e.target.value)} style={{ width: 180 }} />
            <Input.Search placeholder="搜索外部ID/名称/邮箱/手机号" allowClear value={externalKeyword} onChange={e => setExternalKeyword(e.target.value)} onSearch={loadExternalUsers} style={{ width: isMobile ? '100%' : 280 }} />
            <Button icon={<ReloadOutlined />} onClick={loadExternalUsers}>查询</Button>
            <Button type="primary" icon={<ImportOutlined />} onClick={() => { importForm.resetFields(); setImportModalOpen(true); }}>导入外部用户</Button>
          </Space>
          <Table dataSource={externalUsers} columns={externalColumns} rowKey="id" loading={externalLoading} size={isMobile ? 'small' : 'middle'} scroll={{ x: 980 }} pagination={{ pageSize: 10 }} />
        </Card> },
        { key: 'identities', label: '第三方身份', children: <Card>
          <Space wrap style={{ marginBottom: 16 }}>
            <Input placeholder="provider: wechat/feishu" allowClear value={identityProvider} onChange={e => setIdentityProvider(e.target.value)} style={{ width: 220 }} />
            <Input.Search placeholder="搜索第三方ID/union/open/名称/手机号" allowClear value={identityKeyword} onChange={e => setIdentityKeyword(e.target.value)} onSearch={loadIdentities} style={{ width: isMobile ? '100%' : 320 }} />
            <Button icon={<ReloadOutlined />} onClick={loadIdentities}>查询</Button>
          </Space>
          <Table dataSource={identities} columns={identityColumns} rowKey="id" loading={identityLoading} size={isMobile ? 'small' : 'middle'} scroll={{ x: 980 }} pagination={{ pageSize: 10 }} />
        </Card> },
        { key: 'apps', label: '应用接入', children: <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Card>
            <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, flexDirection: isMobile ? 'column' : 'row' }}>
              <div style={{ flex: 1 }}>
                <Title level={4} style={{ marginTop: 0 }}>业务系统接入 AI 助手</Title>
                <Text type="secondary">面向业务系统开发者：后端换短期 Token，前端嵌入 iframe/SDK；EASP 统一治理权限、工具、审计和记忆。</Text>
              </div>
              <Space wrap>
                <Button icon={<ReloadOutlined />} onClick={loadEmbedApps}>刷新</Button>
                <Button type="primary" icon={<PlusOutlined />} onClick={() => { appForm.resetFields(); appForm.setFieldsValue({ token_ttl_seconds: 7200 }); setAppModalOpen(true); }}>新建接入应用</Button>
              </Space>
            </div>
            <Steps
              size="small"
              current={embedApps.length ? 1 : 0}
              style={{ marginTop: 20 }}
              direction={isMobile ? 'vertical' : 'horizontal'}
              items={[
                { title: '创建应用', description: '生成 app_id / app_secret' },
                { title: '后端换 Token', description: '业务系统服务端签名调用 EASP' },
                { title: '前端嵌入', description: 'iframe 或 JS SDK 加载助手' },
                { title: '联调检测', description: '检查租户、来源和外部用户' },
              ]}
            />
          </Card>

          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
            <Card size="small"><Statistic title="接入应用" value={appSummary.total} prefix={<ApiOutlined />} /></Card>
            <Card size="small"><Statistic title="启用中" value={appSummary.active} valueStyle={{ color: '#52c41a' }} prefix={<CheckCircleOutlined />} /></Card>
            <Card size="small"><Statistic title="外部系统" value={appSummary.systems} prefix={<CloudServerOutlined />} /></Card>
          </div>

          <Card title="接入应用工作台" extra={<Text type="secondary">优先从“接入指南”和“联调检测”开始</Text>}>
            {embedApps.length === 0 ? <Empty description="暂无接入应用，先创建一个业务系统应用" image={Empty.PRESENTED_IMAGE_SIMPLE}>
              <Button type="primary" icon={<PlusOutlined />} onClick={() => { appForm.resetFields(); appForm.setFieldsValue({ token_ttl_seconds: 7200 }); setAppModalOpen(true); }}>新建接入应用</Button>
            </Empty> : <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(2, minmax(0, 1fr))', gap: 12 }}>
              {embedApps.map(renderAppCard)}
            </div>}
          </Card>

          <Card title="应用明细">
            <Table dataSource={embedApps} columns={appColumns} rowKey="id" loading={appLoading} size={isMobile ? 'small' : 'middle'} scroll={{ x: 1000 }} pagination={{ pageSize: 10 }} />
          </Card>
        </Space> },
      ]} />

      <AccessManualModal type="user" open={manualOpen} tenantId={currentTenant} onClose={() => setManualOpen(false)} />

      <Modal title="新建用户" open={modalOpen} onOk={onCreate} onCancel={() => setModalOpen(false)} width={isMobile ? '90%' : 460}>
        <Alert type="info" showIcon message="创建租户用户" description="注册/创建会由后端校验租户状态、到期时间和用户上限；创建后建议立即分配租户角色，否则用户可能无法获得菜单、工具和 Skill 权限。" style={{ marginBottom: 16 }} />
        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="account" label="登录账号" rules={[{ required: true, message: '请输入登录账号' }]} tooltip="租户内唯一，不随邮箱/手机号变更"><Input placeholder="zhangsan" /></Form.Item>
          <Form.Item name="email" label="邮箱（属性）" rules={[{ type: 'email' }]}><Input /></Form.Item>
          <Form.Item name="phone" label="手机号（属性）"><Input /></Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }, { min: 6 }]}><Input.Password /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input /></Form.Item>
        </Form>
      </Modal>

      <Modal title="导入外部用户" open={importModalOpen} onOk={onImportExternalUser} onCancel={() => setImportModalOpen(false)} width={isMobile ? '94%' : 760}>
        <Alert type="warning" showIcon message="必传项与登录账号" description="必传 external_system + external_user_id。若需要用户通过 EASP 登录页账号密码登录，必须提供 account，并配置 password 或 default_password；email/phone 只是用户属性，可为空、可重复、可后续修改。" style={{ marginBottom: 16 }} />
        <Form form={importForm} layout="vertical">
          <Space.Compact block>
            <Form.Item name="external_system" label="外部系统" rules={[{ required: true }]} tooltip="必传。与嵌入接入应用 external_system 保持一致，如 crm" style={{ width: '50%' }}><Input placeholder="crm" /></Form.Item>
            <Form.Item name="external_user_id" label="外部用户ID" rules={[{ required: true }]} tooltip="必传。业务系统内稳定用户 ID，用于 token exchange 映射 EASP 用户" style={{ width: '50%' }}><Input placeholder="u_10001" /></Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="account" label="登录账号 account" tooltip="租户内唯一；不传则默认使用 external_user_id 作为账号" style={{ width: '50%' }}><Input placeholder="zhangsan" /></Form.Item>
            <Form.Item name="user_uid" label="用户唯一标识 user_uid" tooltip="建议传。默认可用 {external_system}:{external_user_id}" style={{ width: '50%' }}><Input placeholder="crm:u_10001" /></Form.Item>
          </Space.Compact>
          <Form.Item name="display_name" label="显示名称"><Input placeholder="张三" /></Form.Item>
          <Space.Compact block>
            <Form.Item name="email" label="邮箱（属性）" tooltip="仅作为用户资料，不参与登录账号唯一性校验" style={{ width: '50%' }}><Input placeholder="zhangsan@example.com" /></Form.Item>
            <Form.Item name="phone" label="手机号（属性）" tooltip="仅作为用户资料，不参与登录账号唯一性校验" style={{ width: '50%' }}><Input placeholder="13800000000" /></Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="default_password" label="批量默认密码" rules={[{ min: 6, message: '密码至少6位' }]} tooltip="用于本次导入中未单独填写 password 的用户；批量 API 导入时同样生效" style={{ width: '50%' }}><Input.Password placeholder="本次导入统一密码" /></Form.Item>
            <Form.Item name="password" label="当前用户密码" rules={[{ min: 6, message: '密码至少6位' }]} tooltip="优先级高于批量默认密码，仅作用于当前用户" style={{ width: '50%' }}><Input.Password placeholder="可覆盖默认密码" /></Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="department" label="部门" style={{ width: '50%' }}><Input /></Form.Item>
            <Form.Item name="position" label="岗位" style={{ width: '50%' }}><Input /></Form.Item>
          </Space.Compact>
          <Space.Compact block>
            <Form.Item name="wechat_id" label="微信用户ID" style={{ width: '50%' }}><Input /></Form.Item>
            <Form.Item name="feishu_id" label="飞书用户ID" style={{ width: '50%' }}><Input /></Form.Item>
          </Space.Compact>
          <Form.Item name="avatar" label="头像 URL"><Input /></Form.Item>
          <Form.Item name="profile" label="Profile JSON"><Input.TextArea rows={3} placeholder='{"department":"销售部","level":"L2"}' /></Form.Item>
          <Form.Item name="attributes" label="Attributes JSON"><Input.TextArea rows={3} placeholder='{"region":"华北"}' /></Form.Item>
        </Form>
      </Modal>

      <Modal title="新建接入应用" open={appModalOpen} onOk={onCreateEmbedApp} onCancel={() => setAppModalOpen(false)} width={isMobile ? '94%' : 720}>
        <Alert type="info" showIcon message="创建后会一次性展示 App Secret，请立即复制到业务系统服务端。" description="前端页面永远不要保存 App Secret；业务前端只使用业务后端换回的短期 easp-api-token。" style={{ marginBottom: 16 }} />
        <Form form={appForm} layout="vertical">
          <Form.Item name="name" label="应用名称" rules={[{ required: true }]} tooltip="展示给运营/开发者识别的名称"><Input placeholder="CRM H5 助手" /></Form.Item>
          <Form.Item name="external_system" label="外部系统标识" rules={[{ required: true }]} tooltip="用于外部用户映射和审计归因，建议用稳定英文标识"><Input placeholder="crm" /></Form.Item>
          <Form.Item name="allowed_origins" label="允许来源 Origin（一行一个，留空表示不限制）" tooltip="建议生产环境填写业务系统域名，例如 https://crm.example.com"><Input.TextArea rows={3} placeholder="https://crm.example.com" /></Form.Item>
          <Form.Item name="allowed_scopes" label="允许范围（一行一个，可选）" tooltip="用于限制 token 使用范围，默认可先填 assistant:chat"><Input.TextArea rows={2} placeholder="assistant:chat" /></Form.Item>
          <Form.Item name="token_ttl_seconds" label="Token 有效期（秒）" tooltip="短期 token 建议 30 分钟到 2 小时"><InputNumber min={300} max={86400} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>

      <Modal title={`接入指南 — ${selectedApp?.name || ''}`} open={guideOpen} onCancel={() => setGuideOpen(false)} footer={null} width={isMobile ? '94%' : 960} loading={guideLoading}>
        {guide && <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Alert type="warning" showIcon message="App Secret 只允许放在业务系统服务端；前端只接收短期 easp-api-token。" />
          <Steps
            direction={isMobile ? 'vertical' : 'horizontal'}
            size="small"
            current={1}
            items={[
              { title: '保存密钥', description: '创建时一次性展示 app_secret' },
              { title: '服务端换 Token', description: '签名调用 token exchange' },
              { title: '前端嵌入', description: 'iframe / SDK 加载助手' },
              { title: '运行联调', description: '检查来源和外部用户' },
            ]}
          />
          <Descriptions column={isMobile ? 1 : 2} size="small" bordered>
            <Descriptions.Item label="App ID"><Text copyable code>{guide.app_id}</Text></Descriptions.Item>
            <Descriptions.Item label="Tenant ID"><Text copyable code>{guide.tenant_id}</Text></Descriptions.Item>
            <Descriptions.Item label="外部系统"><Tag color="blue">{guide.external_system}</Tag></Descriptions.Item>
            <Descriptions.Item label="Token TTL">{guide.token_ttl_seconds}s</Descriptions.Item>
            <Descriptions.Item label="Token Exchange" span={isMobile ? 1 : 2}><Text copyable code>{guide.endpoints.token_exchange}</Text></Descriptions.Item>
          </Descriptions>
          <StepCodeBlock title="1. 业务系统后端签名 Payload" note="timestamp/nonce/body 参与 HMAC 签名；app_secret 只在服务端使用。" code={JSON.stringify(guide.examples.signature_payload, null, 2)} />
          <StepCodeBlock title="2. iframe 嵌入代码" note="业务前端只向自己的后端拿短期 token，再加载 EASP 助手页面。" code={guide.examples.iframe} />
          <StepCodeBlock title="3. JS SDK 嵌入代码" note="适合需要按钮/浮窗式集成的业务页面。" code={guide.examples.sdk} />
          <Alert type="info" showIcon message="下一步" description="复制示例完成业务系统后端换 Token 后，回到应用卡片点击“联调检测”，检查来源白名单、租户状态和外部用户导入情况。" />
        </Space>}
      </Modal>

      <Modal title={`联调检测 — ${selectedApp?.name || ''}`} open={diagnoseOpen} onOk={runDiagnose} onCancel={() => setDiagnoseOpen(false)} confirmLoading={diagnoseLoading} okText="开始检测" width={isMobile ? '94%' : 760}>
        <Form form={diagnoseForm} layout="vertical">
          <Form.Item name="origin" label="业务系统 Origin" tooltip="例如 https://crm.example.com。留空只检查应用/租户/外部用户。"><Input placeholder="https://crm.example.com" /></Form.Item>
          <Form.Item name="external_user_id" label="外部用户ID" rules={[{ required: true, message: '请输入要检测的 external_user_id' }]}><Input placeholder="当前业务系统用户ID，如 u10086" /></Form.Item>
        </Form>
        {diagnoseResult && <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(3, minmax(0, 1fr))', gap: 12 }}>
            <Card size="small"><Statistic title="检查项" value={diagnoseResult.checks.length} /></Card>
            <Card size="small"><Statistic title="已通过" value={diagnoseResult.checks.filter(item => item.ok).length} valueStyle={{ color: '#52c41a' }} /></Card>
            <Card size="small"><Statistic title="待修复" value={diagnoseResult.checks.filter(item => !item.ok).length} valueStyle={{ color: diagnoseResult.can_issue_token ? '#52c41a' : '#ff4d4f' }} /></Card>
          </div>
          <Alert type={diagnoseResult.can_issue_token ? 'success' : 'error'} showIcon message={diagnoseResult.can_issue_token ? '前置条件通过，可以换取嵌入式 Token' : '暂不能换取 Token，请按检查项修复'} />
          {diagnoseResult.checks.map((item, index) => <Card key={item.key} size="small" style={{ borderColor: item.ok ? '#b7eb8f' : '#ffccc7' }}>
            <Space align="start">
              {item.ok ? <CheckCircleOutlined style={{ color: '#52c41a', marginTop: 4 }} /> : <CloseCircleOutlined style={{ color: '#ff4d4f', marginTop: 4 }} />}
              <div style={{ flex: 1 }}>
                <Space wrap><Text strong>{index + 1}. {item.label}</Text><Tag color={item.ok ? 'green' : 'red'}>{item.ok ? 'OK' : item.code}</Tag></Space>
                {!item.ok ? <div style={{ marginTop: 6 }}><Text type="secondary">修复建议：{item.suggestion}</Text></div> : <div style={{ marginTop: 6 }}><Text type="secondary">已满足要求</Text></div>}
              </div>
            </Space>
          </Card>)}
        </Space>}
      </Modal>

      <Modal title="用户详情" open={detailOpen} onCancel={() => setDetailOpen(false)} footer={null} width={isMobile ? '94%' : 860} loading={detailLoading}>
        {detail.user && <Space direction="vertical" style={{ width: '100%' }} size="middle">
          <Alert type={userRisk(detail.user as UserRow).label === '正常' ? 'success' : 'warning'} showIcon message={`账号状态：${userRisk(detail.user as UserRow).label}`} description={userRisk(detail.user as UserRow).reason} />
          <Descriptions title="账号主体" bordered size="small" column={1}>
            <Descriptions.Item label="用户ID"><Text copyable code>{detail.user.id}</Text></Descriptions.Item>
            <Descriptions.Item label="登录账号"><Text copyable code>{detail.user.account || '-'}</Text></Descriptions.Item>
            <Descriptions.Item label="用户唯一标识"><Text copyable code>{detail.user.user_uid || '-'}</Text></Descriptions.Item>
            <Descriptions.Item label="邮箱/手机号（属性）">{detail.user.email || '-'} / {detail.user.phone || '-'}</Descriptions.Item>
            <Descriptions.Item label="显示名">{detail.user.display_name || '-'}</Descriptions.Item>
            <Descriptions.Item label="状态"><Space><Tag color={detail.user.status === 'active' ? 'green' : 'red'}>{detail.user.status}</Tag>{detail.user.deleted_at && <Tag color="default">已注销</Tag>}</Space></Descriptions.Item>
            <Descriptions.Item label="登录情况">{detail.user.login_count || 0} 次 / 最近登录：{fmtTime(detail.user.last_login_at)}</Descriptions.Item>
            <Descriptions.Item label="创建/删除时间">{fmtTime(detail.user.created_at)} / {fmtTime(detail.user.deleted_at)}</Descriptions.Item>
          </Descriptions>
          <Descriptions title="扩展属性" bordered size="small" column={1}>
            <Descriptions.Item label="Profile"><Paragraph style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>{parseMaybeJson(detail.user.profile)}</Paragraph></Descriptions.Item>
            <Descriptions.Item label="Attributes"><Paragraph style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>{parseMaybeJson(detail.user.attributes)}</Paragraph></Descriptions.Item>
          </Descriptions>
          <Title level={5}>第三方身份关联</Title>
          <Table dataSource={detail.identities} columns={identityColumns.slice(0, 6)} rowKey="id" size="small" pagination={false} scroll={{ x: 760 }} />
        </Space>}
      </Modal>

      <Modal title={<Space><SafetyCertificateOutlined /><span>角色管理 — {roleModalUser?.account || roleModalUser?.email}</span></Space>} open={roleModalOpen} onOk={saveRoles} onCancel={() => setRoleModalOpen(false)} confirmLoading={roleSaving} okText="保存" cancelText="取消" width={isMobile ? '90%' : 520}>
        {roleModalUser && <div>
          <Descriptions size="small" column={1} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="用户">{roleModalUser.display_name || roleModalUser.account || roleModalUser.email}</Descriptions.Item>
            <Descriptions.Item label="登录账号">{roleModalUser.account || '-'}</Descriptions.Item>
            <Descriptions.Item label="邮箱">{roleModalUser.email || '-'}</Descriptions.Item>
          </Descriptions>
          <div style={{ border: '1px solid #f0f0f0', borderRadius: 8, padding: 16, maxHeight: 320, overflow: 'auto' }}>
            {tenantRoles.length === 0 ? <Text type="secondary">暂无可分配的角色</Text> : <Space direction="vertical" style={{ width: '100%' }}>
              {tenantRoles.map(role => <div key={role.id} style={{ padding: '8px 12px', borderRadius: 6, background: userRoleIds.includes(role.id) ? '#f6ffed' : 'transparent', border: userRoleIds.includes(role.id) ? '1px solid #b7eb8f' : '1px solid transparent' }}>
                <Checkbox checked={userRoleIds.includes(role.id)} onChange={(e) => setUserRoleIds(e.target.checked ? [...userRoleIds, role.id] : userRoleIds.filter(id => id !== role.id))}>
                  <div><div style={{ fontWeight: 500 }}>{role.name}</div>{role.description && <div style={{ fontSize: 12, color: '#999' }}>{role.description}</div>}</div>
                </Checkbox>
                {userRoleIds.includes(role.id) && <Badge status="success" style={{ marginLeft: 8 }} />}
              </div>)}
            </Space>}
          </div>
        </div>}
      </Modal>

      <Modal title={<Space><KeyOutlined /><span>重置密码 — {resetUser?.account || resetUser?.email}</span></Space>} open={resetModalOpen} onCancel={() => setResetModalOpen(false)} width={isMobile ? '90%' : 480} footer={resetSaved ? [<Button key="close" onClick={() => setResetModalOpen(false)}>关闭</Button>] : [<Button key="cancel" onClick={() => setResetModalOpen(false)}>取消</Button>, <Button key="confirm" type="primary" loading={resetLoading} onClick={confirmResetPassword}>确认保存</Button>]}>
        {resetLoading && !resetPassword ? <div style={{ textAlign: 'center', padding: 24 }}>生成中...</div> : resetPassword ? <div>
          <Text type="secondary">已为用户生成随机密码，请复制并妥善保存。点击「确认保存」后密码生效。</Text>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '12px 16px', background: '#f6ffed', border: '1px solid #b7eb8f', borderRadius: 8, marginTop: 12, marginBottom: 12, flexDirection: isMobile ? 'column' : 'row' }}>
            <Text strong style={{ fontSize: isMobile ? 14 : 18, fontFamily: 'monospace', flex: 1, letterSpacing: 2, wordBreak: 'break-all' }}>{resetPassword}</Text>
            <Button icon={<CopyOutlined />} onClick={() => copyText(resetPassword)} size={isMobile ? 'small' : 'middle'}>复制</Button>
          </div>
          {resetSaved && <div style={{ color: '#52c41a', fontWeight: 500 }}>✅ 密码已保存生效，用户可用此密码登录</div>}
        </div> : null}
      </Modal>
    </div>
  );
};

export default Users;
