import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Space, Typography, Popconfirm, App, Switch, Tag, Dropdown, Drawer, Descriptions, Progress, Divider, Alert, Card, Row, Col, Statistic } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, MoreOutlined, BarChartOutlined, ApartmentOutlined, CheckCircleOutlined, WarningOutlined, ClockCircleOutlined, EyeOutlined } from '@ant-design/icons';
import type { Tenant } from '../api/tenant';
import { tenantApi } from '../api/tenant';
import type { UsageStats, ModelUsageStats } from '../api/usage';
import { usageApi } from '../api/usage';
import { userApi } from '../api/user';

const { Title, Text } = Typography;

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
  const [userCounts, setUserCounts] = useState<Record<string, number>>({});
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailTenant, setDetailTenant] = useState<Tenant | null>(null);
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    setLoading(true);
    try {
      const res = await tenantApi.list();
      const list = res.data || [];
      setTenants(list);
      const countEntries = await Promise.all(list.map(async (tenant) => {
        try {
          const users = await userApi.listByTenant(tenant.id, { limit: 500 });
          return [tenant.id, (users.data || []).filter(user => !user.deleted_at).length] as const;
        } catch {
          return [tenant.id, 0] as const;
        }
      }));
      setUserCounts(Object.fromEntries(countEntries));
    }
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

  const tenantRisk = (tenant: Tenant) => {
    if (tenant.status !== 'active') return { label: '已停用', color: 'red', reason: '租户状态非 active，登录、注册和嵌入式助手应由后端拒绝。' };
    if (tenant.expires_at) {
      const days = Math.ceil((new Date(tenant.expires_at).getTime() - Date.now()) / 86400000);
      if (days < 0) return { label: '已过期', color: 'red', reason: '租户已过期，登录/注册应被后端拦截。' };
      if (days <= 30) return { label: '即将过期', color: 'orange', reason: `${days} 天后到期，建议提前续费或调整到期时间。` };
    }
    const max = tenant.max_users || 0;
    const used = userCounts[tenant.id] || 0;
    if (max > 0 && used >= max) return { label: '容量已满', color: 'red', reason: '当前用户数已达到上限，新用户注册/创建应被后端拦截。' };
    if (max > 0 && used / max >= 0.8) return { label: '容量紧张', color: 'orange', reason: '用户容量超过 80%，建议扩容或清理无效账号。' };
    return { label: '正常', color: 'green', reason: '租户状态、到期时间和用户容量正常。' };
  };

  const capacityPercent = (tenant: Tenant) => tenant.max_users > 0 ? Math.min(100, Math.round(((userCounts[tenant.id] || 0) / tenant.max_users) * 100)) : 0;
  const fmtTime = (value?: string | null) => value ? new Date(value).toLocaleString() : '-';
  const activeTenants = tenants.filter(t => t.status === 'active' && tenantRisk(t).label !== '已过期');
  const expiringTenants = tenants.filter(t => ['即将过期', '已过期'].includes(tenantRisk(t).label));
  const capacityRiskTenants = tenants.filter(t => ['容量紧张', '容量已满'].includes(tenantRisk(t).label));
  const unlimitedTenants = tenants.filter(t => !t.max_users || t.max_users === 0);

  const openDetail = (tenant: Tenant) => {
    setDetailTenant(tenant);
    setDetailOpen(true);
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
    { title: '名称', dataIndex: 'name', key: 'name', render: (v: string, record: Tenant) => <Space direction="vertical" size={2}><Space wrap><Text strong>{v}</Text><Tag color={tenantRisk(record).color}>{tenantRisk(record).label}</Tag></Space><Text type="secondary" style={{ fontSize: 12 }}>{record.id}</Text></Space> },
    ...(!isMobile ? [
      { title: '套餐', dataIndex: 'plan', key: 'plan' },
    ] : []),
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => v === 'active' ? <Tag color="green">正常</Tag> : <Tag color="red">{v}</Tag> },
    { title: '到期', key: 'expires_at', render: (_: unknown, record: Tenant) => getExpiryStatus(record) },
    ...(!isMobile ? [
      { title: '用户容量', dataIndex: 'max_users', key: 'max_users', render: (_: number, record: Tenant) => record.max_users === 0 ? <Tag color="blue">不限</Tag> : <Space direction="vertical" size={2} style={{ width: 120 }}><Text>{userCounts[record.id] || 0} / {record.max_users}</Text><Progress percent={capacityPercent(record)} size="small" status={capacityPercent(record) >= 100 ? 'exception' : capacityPercent(record) >= 80 ? 'active' : 'normal'} /></Space> },
      { title: '创建时间', dataIndex: 'created_at', key: 'created_at', render: fmtTime },
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 60 : 220, render: (_: unknown, record: Tenant) => (
      isMobile ? (
        <Dropdown menu={{ items: [
          { key: 'detail', label: '详情', icon: <EyeOutlined />, onClick: () => openDetail(record) },
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
          <Button size="small" icon={<EyeOutlined />} onClick={() => openDetail(record)}>详情</Button>
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
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>租户运营与容量工作台</Title>
          <Text type="secondary">统一查看租户状态、到期风险、用户容量、限流配额和用量消耗；登录/注册/用户上限由后端最终校验。</Text>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); form.resetFields(); setIsPermanent(true); setModalOpen(true); }}>新建租户</Button>
      </div>

      <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
        <Alert type="info" showIcon message="租户是 SaaS 隔离与商业化计费边界" description="到期时间为空表示永久；最大用户数 0 表示不限。前端展示风险和容量，真正的租户状态、到期、用户上限校验由后端在登录/注册/创建用户链路执行。" />
        <Row gutter={[16, 16]}>
          <Col xs={12} lg={6}><Card><Statistic title="租户总数" value={tenants.length} prefix={<ApartmentOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="正常租户" value={activeTenants.length} valueStyle={{ color: '#52c41a' }} prefix={<CheckCircleOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="到期风险" value={expiringTenants.length} valueStyle={{ color: expiringTenants.length ? '#faad14' : '#52c41a' }} prefix={<ClockCircleOutlined />} /></Card></Col>
          <Col xs={12} lg={6}><Card><Statistic title="容量风险" value={capacityRiskTenants.length} valueStyle={{ color: capacityRiskTenants.length ? '#faad14' : '#52c41a' }} prefix={<WarningOutlined />} /></Card></Col>
        </Row>
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={8}><Card size="small" title="用户容量"><Space direction="vertical" size={4}><Text>不限用户租户：<Text strong>{unlimitedTenants.length}</Text></Text><Text>当前活跃用户合计：<Text strong>{Object.values(userCounts).reduce((sum, count) => sum + count, 0)}</Text></Text><Text type="secondary">容量达到上限时，后端会拒绝新增用户。</Text></Space></Card></Col>
          <Col xs={24} lg={8}><Card size="small" title="生命周期"><Space direction="vertical" size={4}><Text>永久租户：<Text strong>{tenants.filter(t => !t.expires_at).length}</Text></Text><Text>非 active：<Text strong>{tenants.filter(t => t.status !== 'active').length}</Text></Text><Text type="secondary">到期或停用会影响登录、注册和嵌入助手。</Text></Space></Card></Col>
          <Col xs={24} lg={8}><Card size="small" title="配额治理"><Space direction="vertical" size={4}><Text>有限流租户：<Text strong>{tenants.filter(t => (t.rate_limit || 0) > 0).length}</Text></Text><Text>有 Token 配额：<Text strong>{tenants.filter(t => (t.daily_token_quota || 0) > 0 || (t.monthly_token_quota || 0) > 0).length}</Text></Text><Text type="secondary">用量抽屉可查看 API 调用和 Token 消耗。</Text></Space></Card></Col>
        </Row>
      </Space>
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
        <Alert
          type={editing ? 'info' : 'warning'}
          showIcon
          style={{ marginBottom: 16 }}
          message={editing ? '修改租户运营边界' : '创建租户会同时创建租户管理员账号'}
          description={editing ? '状态、到期时间、用户上限和配额会影响登录、注册、创建用户和助手调用；保存后由后端执行链统一生效。' : '管理员邮箱和密码只用于初始化该租户的管理员。建议设置明确用户上限和到期时间，便于 B2B SaaS 商业化治理。'}
        />
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
        title={`租户详情 — ${detailTenant?.name || ''}`}
        open={detailOpen}
        onClose={() => { setDetailOpen(false); setDetailTenant(null); }}
        width={isMobile ? '100%' : 680}
      >
        {detailTenant && <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert type={tenantRisk(detailTenant).label === '正常' ? 'success' : 'warning'} showIcon message={`运营状态：${tenantRisk(detailTenant).label}`} description={tenantRisk(detailTenant).reason} />
          <Descriptions title="租户主体" bordered size="small" column={1}>
            <Descriptions.Item label="租户ID"><Text copyable code>{detailTenant.id}</Text></Descriptions.Item>
            <Descriptions.Item label="名称">{detailTenant.name}</Descriptions.Item>
            <Descriptions.Item label="套餐">{detailTenant.plan || '-'}</Descriptions.Item>
            <Descriptions.Item label="状态"><Tag color={detailTenant.status === 'active' ? 'green' : 'red'}>{detailTenant.status}</Tag></Descriptions.Item>
            <Descriptions.Item label="到期时间">{detailTenant.expires_at ? fmtTime(detailTenant.expires_at) : '永久有效'}</Descriptions.Item>
            <Descriptions.Item label="创建/更新时间">{fmtTime(detailTenant.created_at)} / {fmtTime(detailTenant.updated_at)}</Descriptions.Item>
          </Descriptions>
          <Descriptions title="用户容量" bordered size="small" column={1}>
            <Descriptions.Item label="当前用户数">{userCounts[detailTenant.id] || 0}</Descriptions.Item>
            <Descriptions.Item label="用户上限">{detailTenant.max_users === 0 ? '不限' : detailTenant.max_users}</Descriptions.Item>
            <Descriptions.Item label="容量使用">
              {detailTenant.max_users === 0 ? <Tag color="blue">不限</Tag> : <Progress percent={capacityPercent(detailTenant)} status={capacityPercent(detailTenant) >= 100 ? 'exception' : capacityPercent(detailTenant) >= 80 ? 'active' : 'normal'} />}
            </Descriptions.Item>
          </Descriptions>
          <Descriptions title="限流与配额" bordered size="small" column={1}>
            <Descriptions.Item label="每分钟请求上限">{detailTenant.rate_limit || 0 ? `${detailTenant.rate_limit} 次/分` : '不限'}</Descriptions.Item>
            <Descriptions.Item label="API 调用配额">日 {detailTenant.daily_quota || '不限'} / 月 {detailTenant.monthly_quota || '不限'}</Descriptions.Item>
            <Descriptions.Item label="Token 配额">日 {detailTenant.daily_token_quota || '不限'} / 月 {detailTenant.monthly_token_quota || '不限'}</Descriptions.Item>
          </Descriptions>
          <Alert type="info" showIcon message="运营建议" description="需要排查调用成本时点击表格中的“使用量”；需要调整商业化边界时编辑到期时间、用户上限和配额。" />
        </Space>}
      </Drawer>
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
