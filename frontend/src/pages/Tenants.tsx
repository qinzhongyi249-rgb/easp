import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, Popconfirm, App, Switch, Tag, Dropdown, Drawer, Descriptions, Progress, Divider } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, MoreOutlined, BarChartOutlined } from '@ant-design/icons';
import type { Tenant } from '../api/tenant';
import { tenantApi } from '../api/tenant';
import type { UsageStats, ModelUsageStats } from '../api/usage';
import { usageApi } from '../api/usage';

const { Title } = Typography;

const Tenants: React.FC = () => {
  const { message } = App.useApp();
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Tenant | null>(null);
  const [form] = Form.useForm();
  const [isPermanent, setIsPermanent] = useState(true);
  const [usageDrawerOpen, setUsageDrawerOpen] = useState(false);
  const [usageStats, setUsageStats] = useState<UsageStats | null>(null);
  const [usageLoading, setUsageLoading] = useState(false);
  const [usageTenantName, setUsageTenantName] = useState('');
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    setLoading(true);
    try { const res = await tenantApi.list(); setTenants(res.data || []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);

  const loadUsage = async (tenant: Tenant) => {
    setUsageTenantName(tenant.name);
    setUsageDrawerOpen(true);
    setUsageLoading(true);
    try {
      const res = await usageApi.getStats(tenant.id);
      setUsageStats(res.data);
    } catch {
      message.error('加载使用量失败');
    } finally {
      setUsageLoading(false);
    }
  };

  const onOk = async () => {
    const values = await form.validateFields();
    try {
      const payload: Record<string, unknown> = {
        name: values.name,
        max_users: values.max_users || 0,
        rate_limit: values.rate_limit || 0,
        daily_quota: values.daily_quota || 0,
        monthly_quota: values.monthly_quota || 0,
        daily_token_quota: values.daily_token_quota || 0,
        monthly_token_quota: values.monthly_token_quota || 0,
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
              rate_limit: record.rate_limit || 0,
              daily_quota: record.daily_quota || 0,
              monthly_quota: record.monthly_quota || 0,
              daily_token_quota: record.daily_token_quota || 0,
              monthly_token_quota: record.monthly_token_quota || 0,
              expires_at: record.expires_at ? record.expires_at.split('T')[0] : '',
            });
            setIsPermanent(!record.expires_at);
            setModalOpen(true);
          }},
          { key: 'usage', label: '使用量', icon: <BarChartOutlined />, onClick: () => loadUsage(record) },
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
              rate_limit: record.rate_limit || 0,
              daily_quota: record.daily_quota || 0,
              monthly_quota: record.monthly_quota || 0,
              daily_token_quota: record.daily_token_quota || 0,
              monthly_token_quota: record.monthly_token_quota || 0,
              expires_at: record.expires_at ? record.expires_at.split('T')[0] : '',
            });
            setIsPermanent(!record.expires_at);
            setModalOpen(true);
          }}>编辑</Button>
          <Button size="small" icon={<BarChartOutlined />} onClick={() => loadUsage(record)}>使用量</Button>
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

          <Divider style={{ margin: '12px 0' }}>限流 & 配额</Divider>

          <Form.Item name="rate_limit" label="每分钟请求上限" extra="0 = 不限制">
            <InputNumber min={0} placeholder="120" style={{ width: '100%' }} />
          </Form.Item>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="daily_quota" label="每日调用上限" extra="0 = 不限" style={{ flex: 1 }}>
              <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="monthly_quota" label="每月调用上限" extra="0 = 不限" style={{ flex: 1 }}>
              <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
            </Form.Item>
          </div>

          <div style={{ display: 'flex', gap: 16 }}>
            <Form.Item name="daily_token_quota" label="每日Token上限" extra="0 = 不限" style={{ flex: 1 }}>
              <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="monthly_token_quota" label="每月Token上限" extra="0 = 不限" style={{ flex: 1 }}>
              <InputNumber min={0} placeholder="0" style={{ width: '100%' }} />
            </Form.Item>
          </div>

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
      <Drawer
        title={`使用量统计 — ${usageTenantName}`}
        open={usageDrawerOpen}
        onClose={() => { setUsageDrawerOpen(false); setUsageStats(null); }}
        width={isMobile ? '100%' : 600}
      >
        {usageLoading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>加载中...</div>
        ) : usageStats ? (
          <div>
            <Descriptions column={2} bordered size="small" title="API 调用">
              <Descriptions.Item label="今日调用">{usageStats.today_api_calls}</Descriptions.Item>
              <Descriptions.Item label="本月调用">{usageStats.month_api_calls}</Descriptions.Item>
              <Descriptions.Item label="每分钟限制">
                {usageStats.rate_limit > 0 ? `${usageStats.rate_limit} 次/分` : '不限'}
              </Descriptions.Item>
              <Descriptions.Item label="配额">
                {usageStats.daily_quota > 0 || usageStats.monthly_quota > 0
                  ? `日 ${usageStats.daily_quota || '不限'} / 月 ${usageStats.monthly_quota || '不限'}`
                  : '不限'}
              </Descriptions.Item>
            </Descriptions>

            {(usageStats.daily_quota > 0 || usageStats.monthly_quota > 0) && (
              <div style={{ marginTop: 12 }}>
                {usageStats.daily_quota > 0 && (
                  <div style={{ marginBottom: 8 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: '#666' }}>
                      <span>日配额</span><span>{usageStats.today_api_calls} / {usageStats.daily_quota}</span>
                    </div>
                    <Progress percent={Math.min(100, Math.round(usageStats.today_api_calls / usageStats.daily_quota * 100))}
                      size="small" status={usageStats.today_api_calls >= usageStats.daily_quota ? 'exception' : 'active'} />
                  </div>
                )}
                {usageStats.monthly_quota > 0 && (
                  <div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: '#666' }}>
                      <span>月配额</span><span>{usageStats.month_api_calls} / {usageStats.monthly_quota}</span>
                    </div>
                    <Progress percent={Math.min(100, Math.round(usageStats.month_api_calls / usageStats.monthly_quota * 100))}
                      size="small" status={usageStats.month_api_calls >= usageStats.monthly_quota ? 'exception' : 'active'} />
                  </div>
                )}
              </div>
            )}

            <Descriptions column={2} bordered size="small" title="Token 消耗" style={{ marginTop: 16 }}>
              <Descriptions.Item label="今日输入">{usageStats.today_input_tokens.toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="今日输出">{usageStats.today_output_tokens.toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="今日合计">{usageStats.today_total_tokens.toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="Token配额">
                {usageStats.daily_token_quota > 0 ? usageStats.daily_token_quota.toLocaleString() : '不限'}
              </Descriptions.Item>
              <Descriptions.Item label="本月输入">{usageStats.month_input_tokens.toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="本月输出">{usageStats.month_output_tokens.toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="本月合计" span={2}>{usageStats.month_total_tokens.toLocaleString()}</Descriptions.Item>
            </Descriptions>

            {(usageStats.daily_token_quota > 0 || usageStats.monthly_token_quota > 0) && (
              <div style={{ marginTop: 12 }}>
                {usageStats.daily_token_quota > 0 && (
                  <div style={{ marginBottom: 8 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: '#666' }}>
                      <span>日Token配额</span><span>{usageStats.today_total_tokens.toLocaleString()} / {usageStats.daily_token_quota.toLocaleString()}</span>
                    </div>
                    <Progress percent={Math.min(100, Math.round(usageStats.today_total_tokens / usageStats.daily_token_quota * 100))}
                      size="small" status={usageStats.today_total_tokens >= usageStats.daily_token_quota ? 'exception' : 'active'} />
                  </div>
                )}
                {usageStats.monthly_token_quota > 0 && (
                  <div>
                    <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 12, color: '#666' }}>
                      <span>月Token配额</span><span>{usageStats.month_total_tokens.toLocaleString()} / {usageStats.monthly_token_quota.toLocaleString()}</span>
                    </div>
                    <Progress percent={Math.min(100, Math.round(usageStats.month_total_tokens / usageStats.monthly_token_quota * 100))}
                      size="small" status={usageStats.month_total_tokens >= usageStats.monthly_token_quota ? 'exception' : 'active'} />
                  </div>
                )}
              </div>
            )}

            {usageStats.model_usage && usageStats.model_usage.length > 0 && (
              <div style={{ marginTop: 16 }}>
                <div style={{ fontWeight: 500, marginBottom: 8 }}>按模型分组</div>
                {usageStats.model_usage.map((m: ModelUsageStats, i: number) => (
                  <div key={i} style={{ background: '#fafafa', borderRadius: 6, padding: '8px 12px', marginBottom: 8 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <div>
                        <Tag color="blue">{m.provider || '未知'}</Tag>
                        <span style={{ fontWeight: 500 }}>{m.model}</span>
                      </div>
                      <div style={{ fontSize: 12, color: '#888' }}>
                        今日 {m.today_tokens.toLocaleString()} tokens / {m.today_calls} 次
                      </div>
                    </div>
                    <div style={{ fontSize: 12, color: '#666', marginTop: 4 }}>
                      本月: {m.month_tokens.toLocaleString()} tokens ({m.month_calls} 次)
                      &nbsp;|&nbsp; 输入 {m.month_input_tokens.toLocaleString()} / 输出 {m.month_output_tokens.toLocaleString()}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        ) : null}
      </Drawer>
    </div>
  );
};

export default Tenants;
