import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Select, Typography, Popconfirm, App, Tag, Switch, Tooltip, Alert } from 'antd';
import { PlusOutlined, DeleteOutlined, CopyOutlined, KeyOutlined, CheckCircleOutlined, CloseCircleOutlined, FileTextOutlined } from '@ant-design/icons';
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
    { title: '权限', dataIndex: 'scopes', key: 'scopes',
      render: (v: APIKey['scopes']) => {
        const scopes = normalizeScopes(v);
        if (scopes.length === 0) return <Tag color="green">全部</Tag>;
        return scopes.map(s => <Tag key={s} color="blue">{s}</Tag>);
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
        render: (v: number) => v.toLocaleString() },
      { title: '最后使用', dataIndex: 'last_used_at', key: 'last_used_at',
        render: (v: string | null) => v ? new Date(v).toLocaleString() : '-' },
    ] : []),
    { title: '操作', key: 'action', width: isMobile ? 60 : 100,
      render: (_: unknown, record: APIKey) => (
        <Popconfirm title="确认删除此 API Key？删除后无法恢复。" onConfirm={() => onDelete(record.id)}>
          <Button size="small" danger icon={<DeleteOutlined />}>
            {!isMobile && '删除'}
          </Button>
        </Popconfirm>
      )},
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 12 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}>
          <KeyOutlined style={{ marginRight: 8 }} />API Key 管理
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

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="API Key 用于外部系统接入 Embed API"
        description={
          <span>
            通过 API Key，外部系统（小程序、APP、H5 等）可以直接调用 AI 助手，无需用户登录。
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
            <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
              <Input placeholder="例如：小程序客服、APP 助手" />
            </Form.Item>
            <Form.Item name="scopes" label="权限范围" extra="留空表示全部权限">
              <Select mode="multiple" placeholder="选择权限范围" allowClear>
                <Select.Option value="chat">聊天 (chat)</Select.Option>
                <Select.Option value="sessions">会话管理 (sessions)</Select.Option>
              </Select>
            </Form.Item>
            <Form.Item name="expires_in" label="有效期（天）" extra="0 = 永不过期">
              <Input type="number" min={0} placeholder="0" />
            </Form.Item>
          </Form>
        )}
      </Modal>
    </div>
  );
};

export default APIKeys;
