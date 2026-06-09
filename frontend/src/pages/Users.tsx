import React, { useState, useEffect, useCallback } from 'react';
import { Table, Button, Modal, Form, Input, Space, Typography, Popconfirm, Tag, App, Segmented, Checkbox, Descriptions, Badge } from 'antd';
import { PlusOutlined, DeleteOutlined, LockOutlined, UndoOutlined, SafetyCertificateOutlined, KeyOutlined, CopyOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import type { TenantUser } from '../api/user';
import type { Role } from '../api/role';
import { userApi } from '../api/user';
import { roleApi } from '../api/role';

const { Title, Text } = Typography;
interface LayoutContext { currentTenant: string; }

type UserFilter = 'active' | 'deleted' | 'all';

const Users: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [users, setUsers] = useState<(TenantUser & { is_admin?: boolean; role_names?: string[] })[]>([]);
  const [tenantRoles, setTenantRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [filter, setFilter] = useState<UserFilter>('active');

  // 角色管理弹窗状态
  const [roleModalOpen, setRoleModalOpen] = useState(false);
  const [roleModalUser, setRoleModalUser] = useState<(TenantUser & { is_admin?: boolean }) | null>(null);
  const [userRoleIds, setUserRoleIds] = useState<string[]>([]);
  const [roleSaving, setRoleSaving] = useState(false);

  // 重置密码弹窗状态
  const [resetModalOpen, setResetModalOpen] = useState(false);
  const [resetUser, setResetUser] = useState<(TenantUser & { is_admin?: boolean }) | null>(null);
  const [resetPassword, setResetPassword] = useState('');
  const [resetLoading, setResetLoading] = useState(false);
  const [resetSaved, setResetSaved] = useState(false);

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { 
      const [u, r] = await Promise.all([userApi.listByTenant(currentTenant), roleApi.list(currentTenant)]); 
      setUsers(u.data || []); 
      const data = r.data as unknown as { tenant_roles?: Role[]; system_roles?: Role[] }; 
      setTenantRoles(data.tenant_roles || []); 
    }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const onCreate = async () => {
    const values = await form.validateFields();
    try { await userApi.create(currentTenant, values); message.success('创建成功'); setModalOpen(false); form.resetFields(); load(); }
    catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '创建失败'); }
  };

  const onDelete = async (id: string) => {
    try { await userApi.delete(currentTenant, id); message.success('已删除'); load(); }
    catch (err: unknown) { const e = err as { response?: { data?: { error?: string } } }; message.error(e.response?.data?.error || '删除失败'); }
  };

  const onRestore = async (id: string) => {
    try { await userApi.restore(currentTenant, id); message.success('已恢复'); load(); }
    catch { message.error('恢复失败'); }
  };

  // 重置密码：生成随机密码
  const onResetPassword = useCallback(async (user: TenantUser & { is_admin?: boolean }) => {
    setResetUser(user);
    setResetPassword('');
    setResetSaved(false);
    setResetModalOpen(true);
    setResetLoading(true);
    try {
      const res = await userApi.generateResetPassword(currentTenant, user.id);
      setResetPassword(res.data.password);
    } catch {
      message.error('生成密码失败');
      setResetModalOpen(false);
    } finally {
      setResetLoading(false);
    }
  }, [currentTenant]);

  // 确认保存密码
  const confirmResetPassword = async () => {
    if (!resetUser || !resetPassword) return;
    setResetLoading(true);
    try {
      await userApi.confirmResetPassword(currentTenant, resetUser.id, resetPassword);
      setResetSaved(true);
      message.success('密码已重置');
    } catch {
      message.error('保存密码失败');
    } finally {
      setResetLoading(false);
    }
  };

  // 复制到剪贴板
  const copyPassword = async () => {
    try {
      await navigator.clipboard.writeText(resetPassword);
      message.success('已复制到剪贴板');
    } catch {
      message.error('复制失败，请手动选择复制');
    }
  };

  // 打开角色管理弹窗
  const openRoleModal = useCallback(async (user: TenantUser & { is_admin?: boolean }) => {
    setRoleModalUser(user);
    setRoleModalOpen(true);
    try {
      const res = await userApi.getRoles(user.id);
      const roles = (res.data || []) as Role[];
      setUserRoleIds(roles.map(r => r.id));
    } catch {
      setUserRoleIds([]);
    }
  }, []);

  // 保存角色变更
  const saveRoles = async () => {
    if (!roleModalUser) return;
    setRoleSaving(true);
    try {
      // 获取当前角色
      const res = await userApi.getRoles(roleModalUser.id);
      const currentRoles = (res.data || []) as Role[];
      const currentIds = currentRoles.map(r => r.id);

      // 计算需要新增和移除的
      const toAdd = userRoleIds.filter(id => !currentIds.includes(id));
      const toRemove = currentIds.filter(id => !userRoleIds.includes(id));

      // 批量操作
      await Promise.all([
        ...toAdd.map(roleId => userApi.assignRole(roleModalUser.id, roleId)),
        ...toRemove.map(roleId => userApi.revokeRole(roleModalUser.id, roleId)),
      ]);

      message.success('角色更新成功');
      setRoleModalOpen(false);
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '角色更新失败');
    } finally {
      setRoleSaving(false);
    }
  };

  // 根据筛选条件过滤用户
  const filteredUsers = users.filter(user => {
    if (filter === 'active') return !user.deleted_at;
    if (filter === 'deleted') return !!user.deleted_at;
    return true;
  });

  const columns = [
    { 
      title: '邮箱', 
      dataIndex: 'email', 
      key: 'email',
      render: (v: string, record: TenantUser & { is_admin?: boolean; role_names?: string[] }) => (
        <Space>
          <span style={record.deleted_at ? { textDecoration: 'line-through', color: '#999' } : {}}>{v}</span>
          {record.is_admin && <Tag icon={<LockOutlined />} color="gold">超级管理员</Tag>}
          {!record.is_admin && record.role_names?.includes('管理员') && <Tag icon={<SafetyCertificateOutlined />} color="blue">管理员</Tag>}
          {record.deleted_at && <Tag color="default">已注销</Tag>}
        </Space>
      )
    },
    { title: '显示名', dataIndex: 'display_name', key: 'display_name' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'active' ? 'green' : 'red'}>{v}</Tag> },
    { title: '登录次数', dataIndex: 'login_count', key: 'login_count' },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { 
      title: '操作', 
      key: 'action', 
      width: 220,
      render: (_: unknown, record: TenantUser & { is_admin?: boolean }) => (
        <Space>
          {record.deleted_at ? (
            <Popconfirm title="确认恢复此用户?" onConfirm={() => onRestore(record.id)}>
              <Button size="small" type="primary" icon={<UndoOutlined />}>恢复</Button>
            </Popconfirm>
          ) : record.is_admin ? (
            <Tag color="default">不可操作</Tag>
          ) : (
            <>
              <Button size="small" icon={<SafetyCertificateOutlined />} onClick={() => openRoleModal(record)}>
                角色管理
              </Button>
              <Button size="small" icon={<KeyOutlined />} onClick={() => onResetPassword(record)}>
                重置密码
              </Button>
              <Popconfirm title="确认删除此用户?" onConfirm={() => onDelete(record.id)}>
                <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
              </Popconfirm>
            </>
          )}
        </Space>
      )
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <Title level={3}>用户管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setModalOpen(true); }}>新建用户</Button>
      </div>
      
      <div style={{ marginBottom: 16 }}>
        <Segmented
          options={[
            { label: '正常用户', value: 'active' },
            { label: '已注销用户', value: 'deleted' },
            { label: '全部', value: 'all' },
          ]}
          value={filter}
          onChange={(value) => setFilter(value as UserFilter)}
        />
      </div>

      <Table 
        dataSource={filteredUsers} 
        columns={columns} 
        rowKey="id" 
        loading={loading}
        rowClassName={(record) => record.deleted_at ? 'deleted-row' : ''}
      />
      
      {/* 新建用户弹窗 */}
      <Modal title="新建用户" open={modalOpen} onOk={onCreate} onCancel={() => setModalOpen(false)}>
        <Form form={form} layout="vertical">
          <Form.Item name="email" label="邮箱" rules={[{ required: true }, { type: 'email' }]}><Input /></Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }, { min: 6 }]}><Input.Password /></Form.Item>
          <Form.Item name="display_name" label="显示名称"><Input /></Form.Item>
        </Form>
      </Modal>

      {/* 角色管理弹窗 */}
      <Modal
        title={
          <Space>
            <SafetyCertificateOutlined />
            <span>角色管理 — {roleModalUser?.email}</span>
          </Space>
        }
        open={roleModalOpen}
        onOk={saveRoles}
        onCancel={() => setRoleModalOpen(false)}
        confirmLoading={roleSaving}
        okText="保存"
        cancelText="取消"
        width={500}
      >
        {roleModalUser && (
          <div>
            <Descriptions size="small" column={1} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="用户">{roleModalUser.display_name || roleModalUser.email}</Descriptions.Item>
              <Descriptions.Item label="邮箱">{roleModalUser.email}</Descriptions.Item>
            </Descriptions>
            
            <div style={{ marginBottom: 8, fontWeight: 500 }}>选择角色：</div>
            <div style={{ border: '1px solid #f0f0f0', borderRadius: 8, padding: 16, maxHeight: 300, overflow: 'auto' }}>
              {tenantRoles.length === 0 ? (
                <Text type="secondary">暂无可分配的角色</Text>
              ) : (
                <Space direction="vertical" style={{ width: '100%' }}>
                  {tenantRoles.map(role => (
                    <div 
                      key={role.id} 
                      style={{ 
                        display: 'flex', 
                        alignItems: 'center', 
                        padding: '8px 12px', 
                        borderRadius: 6,
                        background: userRoleIds.includes(role.id) ? '#f6ffed' : 'transparent',
                        border: userRoleIds.includes(role.id) ? '1px solid #b7eb8f' : '1px solid transparent',
                        transition: 'all 0.2s',
                      }}
                    >
                      <Checkbox
                        checked={userRoleIds.includes(role.id)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setUserRoleIds([...userRoleIds, role.id]);
                          } else {
                            setUserRoleIds(userRoleIds.filter(id => id !== role.id));
                          }
                        }}
                      >
                        <div>
                          <div style={{ fontWeight: 500 }}>{role.name}</div>
                          {role.description && (
                            <div style={{ fontSize: 12, color: '#999', marginTop: 2 }}>{role.description}</div>
                          )}
                        </div>
                      </Checkbox>
                      {userRoleIds.includes(role.id) && (
                        <Badge status="success" style={{ marginLeft: 'auto' }} />
                      )}
                    </div>
                  ))}
                </Space>
              )}
            </div>
            
            <div style={{ marginTop: 12, padding: '8px 12px', background: '#f6f6f6', borderRadius: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                已选择 <Text strong>{userRoleIds.length}</Text> 个角色
              </Text>
            </div>
          </div>
        )}
      </Modal>

      {/* 重置密码弹窗 */}
      <Modal
        title={
          <Space>
            <KeyOutlined />
            <span>重置密码 — {resetUser?.email}</span>
          </Space>
        }
        open={resetModalOpen}
        onCancel={() => setResetModalOpen(false)}
        width={480}
        footer={resetSaved ? [
          <Button key="close" onClick={() => setResetModalOpen(false)}>关闭</Button>
        ] : [
          <Button key="cancel" onClick={() => setResetModalOpen(false)}>取消</Button>,
          <Button key="confirm" type="primary" loading={resetLoading} onClick={confirmResetPassword}>
            确认保存
          </Button>
        ]}
      >
        {resetLoading && !resetPassword ? (
          <div style={{ textAlign: 'center', padding: 24 }}>生成中...</div>
        ) : resetPassword ? (
          <div>
            <div style={{ marginBottom: 16 }}>
              <Text type="secondary">已为用户生成随机密码，请复制并妥善保存。点击「确认保存」后密码生效。</Text>
            </div>
            <div style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '12px 16px', background: '#f6ffed', border: '1px solid #b7eb8f',
              borderRadius: 8, marginBottom: 12
            }}>
              <Text strong style={{ fontSize: 18, fontFamily: 'monospace', flex: 1, letterSpacing: 2 }}>
                {resetPassword}
              </Text>
              <Button icon={<CopyOutlined />} onClick={copyPassword}>复制</Button>
            </div>
            {resetSaved && (
              <div style={{ color: '#52c41a', fontWeight: 500 }}>
                ✅ 密码已保存生效，用户可用此密码登录
              </div>
            )}
          </div>
        ) : null}
      </Modal>
    </div>
  );
};

export default Users;
