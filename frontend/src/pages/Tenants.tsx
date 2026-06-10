import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, Popconfirm, App, Switch, Tag, Dropdown } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, MoreOutlined } from '@ant-design/icons';
import type { Tenant } from '../api/tenant';
import { tenantApi } from '../api/tenant';

const { Title } = Typography;

const Tenants: React.FC = () => {
  const { message } = App.useApp();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Tenant | null>(null);
  const [form] = Form.useForm();
  const [isPermanent, setIsPermanent] = useState(true);
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    setLoading(true);
    try { const res = await tenantApi.list(); setTenants(res.data || []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const onOk = async () => {
    const values = await form.validateFields();
    try {
      const payload: Record<string, unknown> = {
        name: values.name,
        max_users: values.max_users || 0,
      };
      
      if (editing) {
        payload.status = values.status || editing.status;
        if (isPermanent) {
          payload.expires_at = '';
        } else if (values.expires_at) {
          payload.expires_at = values.expires_at;
        }
        await tenantApi.update(editing.id, payload as Partial<Tenant>);
        message.success('更新成功');
      } else {
        payload.admin_email = values.admin_email;
        payload.admin_pass = values.admin_pass;
        if (!isPermanent && values.expires_at) {
          payload.expires_at = values.expires_at;
        }
        await tenantApi.create(payload);
        message.success('创建成功');
      }
      setModalOpen(false); form.resetFields(); setEditing(null); load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '操作失败');
    }
  };

  const onDelete = async (id: string) => {
    try { await tenantApi.delete(id); message.success('删除成功'); load(); }
    catch { message.error('删除失败'); }
  };

  const getExpiryStatus = (record: Tenant) => {
    if (!record.expires_at) {
      return <Tag color="green">永久有效</Tag>;
    }
    const expiry = new Date(record.expires_at);
    const now = new Date();
    if (expiry < now) {
      return <Tag color="red">已过期</Tag>;
    }
    const thirtyDays = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000);
    if (expiry < thirtyDays) {
      return <Tag color="orange">即将过期</Tag>;
    }
    return <Tag color="blue">{record.expires_at.split('T')[0]}</Tag>;
  };

  const columns = [
    ...(!isMobile ? [
      { title: 'ID', dataIndex: 'id', key: 'id', ellipsis: true },
    ] : []),
    { title: '名称', dataIndex: 'name', key: 'name' },
    ...(!isMobile ? [
      { title: '套餐', dataIndex: 'plan', key: 'plan' },
    ] : []),
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => v === 'active' ? <Tag color="green">正常</Tag> : <Tag color="red">{v}</Tag> },
    { title: '到期', key: 'expires_at', render: (_: unknown, record: Tenant) => getExpiryStatus(record) },
    ...(!isMobile ? [
      { title: '用户上限', dataIndex: 'max_users', key: 'max_users', render: (v: number) => v === 0 ? '不限' : v },
      { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 60 : 150, render: (_: unknown, record: Tenant) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'edit', label: '编辑', icon: <EditOutlined />, onClick: () => {
            setEditing(record);
            form.setFieldsValue({
              name: record.name,
              status: record.status,
              max_users: record.max_users,
              expires_at: record.expires_at ? record.expires_at.split('T')[0] : '',
            });
            setIsPermanent(!record.expires_at);
            setModalOpen(true);
          }},
          { key: 'delete', label: '删除', icon: <DeleteOutlined />, danger: true, onClick: () => onDelete(record.id) },
        ]}} trigger={['click']}>
          <Button type="text" icon={<MoreOutlined />} />
        </Dropdown>
      ) : (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => {
            setEditing(record);
            form.setFieldsValue({
              name: record.name,
              status: record.status,
              max_users: record.max_users,
              expires_at: record.expires_at ? record.expires_at.split('T')[0] : '',
            });
            setIsPermanent(!record.expires_at);
            setModalOpen(true);
          }}>编辑</Button>
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
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>租户管理</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setIsPermanent(true); setModalOpen(true); }}>新建租户</Button>
      </div>
      <Table 
        dataSource={tenants} 
        columns={columns} 
        rowKey="id" 
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={isMobile ? { x: 400 } : undefined}
        pagination={isMobile ? { pageSize: 10, size: 'small' } : undefined}
      />
      <Modal 
        title={editing ? '编辑租户' : '新建租户'} 
        open={modalOpen} 
        onOk={onOk} 
        onCancel={() => setModalOpen(false)}
        width={isMobile ? '90%' : 500}
      >
        <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
          <Form.Item name="name" label="租户名称" rules={[{ required: true, message: '请输入租户名称' }]}>
            <Input placeholder="请输入租户名称" />
          </Form.Item>

          {editing && (
            <Form.Item name="status" label="状态">
              <Input placeholder="active / suspended / inactive" />
            </Form.Item>
          )}
          
          <Form.Item label="到期设置">
            <div style={{ display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap' }}>
              <Switch 
                checked={isPermanent} 
                onChange={(checked) => setIsPermanent(checked)}
                checkedChildren="永久有效"
                unCheckedChildren="指定日期"
              />
              {!isPermanent && (
                <Form.Item name="expires_at" noStyle>
                  <Input type="date" style={{ width: isMobile ? '100%' : 200 }} placeholder="选择到期日期" />
                </Form.Item>
              )}
            </div>
          </Form.Item>

          <Form.Item name="max_users" label="最大用户数" extra="0 表示不限制">
            <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
          </Form.Item>

          {!editing && (
            <>
              <div style={{ margin: '16px 0 8px', fontWeight: 500, color: '#1890ff' }}>管理员账号信息</div>
              <Form.Item 
                name="admin_email" 
                label="管理员邮箱" 
                rules={[
                  { required: true, message: '请输入管理员邮箱' },
                  { type: 'email', message: '请输入有效的邮箱地址' }
                ]}
              >
                <Input placeholder="管理员登录邮箱" />
              </Form.Item>
              <Form.Item 
                name="admin_pass" 
                label="管理员密码" 
                rules={[
                  { required: true, message: '请输入管理员密码' },
                  { min: 6, message: '密码至少6位' }
                ]}
              >
                <Input.Password placeholder="管理员登录密码" />
              </Form.Item>
            </>
          )}
        </Form>
      </Modal>
    </div>
  );
};

export default Tenants;
