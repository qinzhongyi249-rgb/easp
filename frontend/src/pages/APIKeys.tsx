import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Select, Typography, Popconfirm, App, Tag, Switch, Tooltip, Alert, Card, Row, Col, Statistic, Space, Drawer, Descriptions, Progress } from 'antd';
import { PlusOutlined, DeleteOutlined, CopyOutlined, KeyOutlined, CheckCircleOutlined, CloseCircleOutlined, FileTextOutlined, WarningOutlined, ClockCircleOutlined, EyeOutlined } from '@ant-design/icons';
import type { APIKey } from '../api/apiKey';
import { apiKeyApi } from '../api/apiKey';
import { useOutletContext } from 'react-router-dom';

const { Title, Text, Paragraph } = Typography;

const normalizeScopes = (value: APIKey['scopes']): string[] => {
  if (Array.isArray(value)) return value.filter((s): s is string => typeof s === 'string' && s.length > 0);
  if (typeof value === 'string' && value.trim()) {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed.filter((s): s is string => typeof s === 'string' && s.length > 0) : [];
    } catch {
      return [];
    }
  }
  return [];
};

const formatTime = (value?: string | null) => value ? new Date(value).toLocaleString() : '-';

const isExpired = (key: APIKey) => !!key.expires_at && new Date(key.expires_at).getTime() < Date.now();
const expiresInDays = (key: APIKey) => key.expires_at ? Math.ceil((new Date(key.expires_at).getTime() - Date.now()) / 86400000) : null;
const unusedDays = (key: APIKey) => key.last_used_at ? Math.floor((Date.now() - new Date(key.last_used_at).getTime()) / 86400000) : null;

const riskOf = (key: APIKey): { level: 'high' | 'medium' | 'low'; label: string; color: string; reason: string } => {
  if (!key.enabled) return { level: 'low', label: '已禁用', color: 'default', reason: '凭证已禁用，不可调用。' };
  if (isExpired(key)) return { level: 'high', label: '已过期', color: 'red', reason: '凭证已过期，应删除或重新签发。' };
  const scopes = normalizeScopes(key.scopes);
  if (scopes.length === 0) return { level: 'high', label: '全部权限', color: 'red', reason: '未限制 scopes，外部系统可使用该入口的全部 API Key 权限。' };
  const days = unusedDays(key);
  if (days === null) return { level: 'medium', label: '未使用', color: 'orange', reason: '创建后尚未使用，建议确认是否仍需要保留。' };
  if (days > 90) return { level: 'medium', label: '长期未用', color: 'orange', reason: '超过 90 天未使用，建议禁用或轮换。' };
  return { level: 'low', label: '正常', color: 'green', reason: '权限范围和最近使用状态正常。' };
};

const scopeLabel = (scope: string) => ({ chat: '聊天', sessions: '会话管理' }[scope] || scope);

interface LayoutContext {
  currentTenant: string;
}

const APIKeys: React.FC = () => {
  const { message } = App.useApp();
  const { currentTenant } = useOutletContext<LayoutContext>();
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [selectedKey, setSelectedKey] = useState<APIKey | null>(null);
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const res = await apiKeyApi.list(currentTenant);
      setKeys(Array.isArray(res.data) ? res.data : []);
    } catch {
      message.error('加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const enabledCount = keys.filter(k => k.enabled && !isExpired(k)).length;
  const highRiskCount = keys.filter(k => riskOf(k).level === 'high').length;
  const neverUsedCount = keys.filter(k => !k.last_used_at).length;
  const scopedCount = keys.filter(k => normalizeScopes(k.scopes).length > 0).length;
  const scopedRate = keys.length ? Math.round((scopedCount / keys.length) * 100) : 0;

  const onCreate = async () => {
    const values = await form.validateFields();
    try {
      const res = await apiKeyApi.create(currentTenant, {
        name: values.name,
        scopes: values.scopes || [],
        expires_in: values.expires_in || 0,
      });
      setCreatedKey(res.data.key || null);
      message.success('API Key 创建成功');
      load();
    } catch (err: unknown) {
      const e = err as { response?: { data?: { error?: string } } };
      message.error(e.response?.data?.error || '创建失败');
    }
  };

  const onDelete = async (id: string) => {
    try {
      await apiKeyApi.delete(currentTenant, id);
      message.success('已删除');
      load();
    } catch {
      message.error('删除失败');
    }
  };

  const onToggle = async (id: string, enabled: boolean) => {
    try {
      await apiKeyApi.toggle(currentTenant, id, enabled);
      message.success(enabled ? '已启用' : '已禁用');
      load();
    } catch {
      message.error('操作失败');
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      message.success('已复制到剪贴板');
    });
  };

  const columns = [
    ...(!isMobile ? [
      { title: 'ID', dataIndex: 'id', key: 'id', ellipsis: true, width: 80,
        render: (v: string) => v.substring(0, 8) + '...' },
    ] : []),
    { title: '绑定用户', key: 'user', render: (_: unknown, record: APIKey) => (
      <div>
        <div style={{ fontWeight: 500 }}>{record.user_display_name || '-'}</div>
        <div style={{ fontSize: 12, color: '#888' }}>{record.user_email || record.user_id}</div>
      </div>
    )},
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: 'Key 前缀', dataIndex: 'key_prefix', key: 'key_prefix',
      render: (v: string) => <Tag>{v}</Tag> },
    { title: '权限边界', dataIndex: 'scopes', key: 'scopes',
      render: (v: APIKey['scopes']) => {
        const scopes = normalizeScopes(v);
        if (scopes.length === 0) return <Tag color="red">全部权限</Tag>;
        return scopes.map(s => <Tag key={s} color="blue">{scopeLabel(s)}</Tag>);
      }},
    { title: '风险', key: 'risk', render: (_: unknown, record: APIKey) => {
      const risk = riskOf(record);
      return <Tooltip title={risk.reason}><Tag color={risk.color}>{risk.label}</Tag></Tooltip>;
    }},
    { title: '状态', key: 'enabled', render: (_: unknown, record: APIKey) => (
      <Switch
        checked={record.enabled}
        onChange={(checked) => onToggle(record.id, checked)}
        checkedChildren={<CheckCircleOutlined />}
        unCheckedChildren={<CloseCircleOutlined />}
        size="small"
      />
    )},
    ...(!isMobile ? [
      { title: '调用次数', dataIndex: 'usage_count', key: 'usage_count',
        render: (v: number) => (v || 0).toLocaleString() },
      { title: '最后使用', dataIndex: 'last_used_at', key: 'last_used_at',
        render: (v: string | null) => formatTime(v) },
      { title: '过期时间', dataIndex: 'expires_at', key: 'expires_at',
        render: (_: string | null, record: APIKey) => {
          const days = expiresInDays(record);
          if (days === null) return <Tag color="orange">永不过期</Tag>;
          if (days < 0) return <Tag color="red">已过期</Tag>;
          return <Tag color={days <= 30 ? 'orange' : 'green'}>{days} 天后</Tag>;
        } },
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 96 : 160,
      render: (_: unknown, record: APIKey) => (
        <Space size={4}>
          <Button size="small" icon={<EyeOutlined />} onClick={() => setSelectedKey(record)}>
            {!isMobile && '详情'}
          </Button>
          <Popconfirm title="确认删除此 API Key？删除后无法恢复。" onConfirm={() => onDelete(record.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>
              {!isMobile && '删除'}
            </Button>
          </Popconfirm>
        </Space>
      )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>
          <KeyOutlined style={{ marginRight: 8 }} />凭证安全工作台
        </Title>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Button icon={<FileTextOutlined />} href="/docs/api-key-access" target="_blank" rel="noopener noreferrer">
            接入文档
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { form.resetFields(); setCreatedKey(null); setModalOpen(true); }}>
            创建 API Key
          </Button>
        </div>
      </div>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={12} lg={6}><Card><Statistic title="API Key 总数" value={keys.length} prefix={<KeyOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="可用凭证" value={enabledCount} prefix={<CheckCircleOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="高风险" value={highRiskCount} valueStyle={{ color: highRiskCount ? '#cf1322' : undefined }} prefix={<WarningOutlined />} /></Card></Col>
        <Col xs={12} lg={6}><Card><Statistic title="从未使用" value={neverUsedCount} prefix={<ClockCircleOutlined />} /></Card></Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>
        <Col xs={24} lg={12}>
          <Card title="权限边界" size="small">
            <Space direction="vertical" style={{ width: '100%' }}>
              <Progress percent={scopedRate} size="small" status={scopedRate < 100 && keys.length ? 'exception' : 'success'} />
              <Text type="secondary">{scopedCount}/{keys.length} 个凭证限制了 scopes；留空代表全部权限，生产环境建议最小权限。</Text>
            </Space>
          </Card>
        </Col>
        <Col xs={24} lg={12}>
          <Card title="轮换建议" size="small">
            <Space wrap>
              <Tag color="red">全部权限：{keys.filter(k => normalizeScopes(k.scopes).length === 0).length}</Tag>
              <Tag color="orange">永不过期：{keys.filter(k => !k.expires_at).length}</Tag>
              <Tag color="blue">30天内过期：{keys.filter(k => { const d = expiresInDays(k); return d !== null && d >= 0 && d <= 30; }).length}</Tag>
            </Space>
          </Card>
        </Col>
      </Row>

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="API Key 用于服务端到服务端接入 Embed API"
        description={
          <span>
            API Key 是外部系统调用嵌入助手的系统级凭证，应放在业务系统后端或可信服务端，不应暴露在 H5/小程序前端。权限边界由 scopes、启用状态、过期时间和后端执行链共同决定。
            <a href="/docs/api-key-access" target="_blank" rel="noopener noreferrer" style={{ marginLeft: 8 }}>查看接入文档 →</a>
          </span>
        }
      />

      <Table
        dataSource={keys}
        columns={columns}
        rowKey="id"
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={isMobile ? { x: 500 } : undefined}
        pagination={isMobile ? { pageSize: 10, size: 'small' } : undefined}
      />

      <Drawer
        title="API Key 安全详情"
        open={!!selectedKey}
        onClose={() => setSelectedKey(null)}
        width={isMobile ? '100%' : 620}
      >
        {selectedKey && (() => {
          const risk = riskOf(selectedKey);
          const scopes = normalizeScopes(selectedKey.scopes);
          return (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Alert type={risk.level === 'high' ? 'error' : risk.level === 'medium' ? 'warning' : 'success'} showIcon message={`风险等级：${risk.label}`} description={risk.reason} />
              <Descriptions bordered column={1} size="small" title="凭证归属">
                <Descriptions.Item label="ID">{selectedKey.id}</Descriptions.Item>
                <Descriptions.Item label="名称">{selectedKey.name}</Descriptions.Item>
                <Descriptions.Item label="Key 前缀"><Tag>{selectedKey.key_prefix}</Tag></Descriptions.Item>
                <Descriptions.Item label="绑定用户">{selectedKey.user_display_name || selectedKey.user_email || selectedKey.user_id}</Descriptions.Item>
                <Descriptions.Item label="状态">{selectedKey.enabled ? <Tag color="green">启用</Tag> : <Tag>禁用</Tag>}</Descriptions.Item>
              </Descriptions>
              <Descriptions bordered column={1} size="small" title="权限与生命周期">
                <Descriptions.Item label="权限边界">
                  {scopes.length ? scopes.map(s => <Tag key={s} color="blue">{scopeLabel(s)} ({s})</Tag>) : <Tag color="red">全部权限</Tag>}
                </Descriptions.Item>
                <Descriptions.Item label="创建时间">{formatTime(selectedKey.created_at)}</Descriptions.Item>
                <Descriptions.Item label="过期时间">{selectedKey.expires_at ? formatTime(selectedKey.expires_at) : <Tag color="orange">永不过期</Tag>}</Descriptions.Item>
                <Descriptions.Item label="最后使用">{formatTime(selectedKey.last_used_at)}</Descriptions.Item>
                <Descriptions.Item label="调用次数">{(selectedKey.usage_count || 0).toLocaleString()}</Descriptions.Item>
              </Descriptions>
              <Alert
                type="warning"
                showIcon
                message="轮换建议"
                description="发现泄露、长期未使用、全部权限、永不过期或绑定用户变更时，应禁用旧 Key 并重新签发。完整密钥只在创建时展示一次，后续只能删除/禁用/重建。"
              />
            </Space>
          );
        })()}
      </Drawer>

      <Modal
        title="创建 API Key"
        open={modalOpen}
        onCancel={() => { setModalOpen(false); setCreatedKey(null); }}
        footer={createdKey ? [
          <Button key="close" onClick={() => { setModalOpen(false); setCreatedKey(null); }}>关闭</Button>
        ] : undefined}
        onOk={createdKey ? undefined : onCreate}
        width={isMobile ? '90%' : 520}
      >
        {createdKey ? (
          <div>
            <Alert
              type="warning"
              showIcon
              message="请保存您的 API Key"
              description="此密钥只会显示一次，请立即复制并妥善保管。"
              style={{ marginBottom: 16 }}
            />
            <div style={{ background: '#f5f5f5', padding: 12, borderRadius: 6, marginBottom: 16 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <Text code style={{ fontSize: 13, wordBreak: 'break-all' }}>{createdKey}</Text>
                <Tooltip title="复制">
                  <Button icon={<CopyOutlined />} onClick={() => copyToClipboard(createdKey)} />
                </Tooltip>
              </div>
            </div>
            <Paragraph type="secondary" style={{ fontSize: 12 }}>
              调用示例：<br />
              <Text code>curl -H "Authorization: Bearer {createdKey.substring(0, 13)}..." https://your-domain/embed/v1/chat</Text>
            </Paragraph>
          </div>
        ) : (
          <Form form={form} layout="vertical" size={isMobile ? 'middle' : 'large'}>
            <Alert
              type="info"
              showIcon
              message="创建前确认权限边界"
              description="建议为不同业务系统分别创建 Key，限制 scopes 并设置过期时间。完整密钥只会在创建成功后展示一次。"
              style={{ marginBottom: 16 }}
            />
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input placeholder="例如：小程序客服、APP 助手" />
            </Form.Item>
            <Form.Item name="scopes" label="权限范围" extra="留空表示全部权限；生产环境建议只选择必要能力。">
              <Select mode="multiple" placeholder="选择权限范围" allowClear>
                <Select.Option value="chat">聊天 (chat)</Select.Option>
                <Select.Option value="sessions">会话管理 (sessions)</Select.Option>
              </Select>
            </Form.Item>
            <Form.Item name="expires_in" label="有效期（天）" extra="0 = 永不过期；生产环境建议设置明确有效期并定期轮换。">
              <Input type="number" min={0} placeholder="0" />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </div>
  );
};

export default APIKeys;
