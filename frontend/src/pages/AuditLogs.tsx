import React, { useState, useEffect } from 'react';
import { Table, Typography, App, Tag, Button, Drawer, Descriptions, Space, Card, Form, Input, Select, Statistic, Alert } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { FileTextOutlined, EyeOutlined, ReloadOutlined, SearchOutlined, ApiOutlined, UserOutlined, ClockCircleOutlined, WarningOutlined, CheckCircleOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title, Text, Paragraph } = Typography;

interface LayoutContext { currentTenant: string; }

interface AuditLog {
  id: string;
  tenant_id: string;
  user_id?: string;
  user_uid?: string;
  agent_id?: string;
  source_type?: string;
  source_app_id?: string;
  external_system?: string;
  external_user_id?: string;
  tool: string;
  action: string;
  resource?: string;
  detail?: string;
  decision?: string;
  result?: string;
  duration_ms?: number;
  ip?: string;
  user_agent?: string;
  created_at: string;
}

interface AuditFilter {
  source_type?: string;
  source_app_id?: string;
  external_system?: string;
  external_user_id?: string;
  user_uid?: string;
  user_id?: string;
  tool?: string;
  action?: string;
}

const safeParseDetail = (value?: string): string => {
  if (!value) return '-';
  let parsed: unknown = value;
  for (let i = 0; i < 3; i += 1) {
    if (typeof parsed !== 'string') break;
    const trimmed = parsed.trim();
    if (!trimmed) return '-';
    try {
      parsed = JSON.parse(trimmed);
    } catch {
      parsed = trimmed;
      break;
    }
  }
  if (typeof parsed === 'string') return parsed;
  try { return JSON.stringify(parsed, null, 2); } catch { return String(parsed); }
};

const getResultColor = (result?: string) => {
  if (!result) return 'default';
  if (['success', 'allow', 'approved'].includes(result)) return 'success';
  if (['failed', 'error', 'deny', 'rejected'].includes(result)) return 'error';
  return 'processing';
};

const getDecisionColor = (decision?: string) => {
  if (!decision) return 'default';
  if (['approved', 'allow'].includes(decision)) return 'green';
  if (['rejected', 'deny'].includes(decision)) return 'red';
  return 'blue';
};

const getSourceColor = (source?: string) => {
  if (source === 'embed') return 'purple';
  if (source === 'api_key') return 'cyan';
  if (source === 'sso') return 'geekblue';
  if (source === 'admin') return 'blue';
  return 'default';
};

const cleanParams = (values: AuditFilter) => {
  const params: Record<string, string | number> = { limit: 100 };
  Object.entries(values).forEach(([key, value]) => {
    if (typeof value === 'string' && value.trim()) params[key] = value.trim();
  });
  return params;
};

const AuditLogs: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [form] = Form.useForm<AuditFilter>();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const isMobile = window.innerWidth < 768;

  const load = async (values?: AuditFilter) => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const params = cleanParams(values || form.getFieldsValue());
      const res = await client.get(`/tenants/${currentTenant}/audit-logs`, { params });
      setLogs(Array.isArray(res.data) ? res.data : []);
    } catch {
      message.error('加载审计日志失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load({}); }, [currentTenant]);

  const isFailure = (log: AuditLog) => {
    const result = (log.result || '').toLowerCase();
    const decision = (log.decision || '').toLowerCase();
    return ['failed', 'error', 'deny', 'rejected'].includes(result) || ['rejected', 'deny'].includes(decision);
  };

  const summary = {
    total: logs.length,
    embed: logs.filter(l => l.source_type === 'embed').length,
    externalIdentity: logs.filter(l => l.external_system || l.external_user_id || l.source_app_id).length,
    failed: logs.filter(isFailure).length,
    withDuration: logs.filter(l => typeof l.duration_ms === 'number').length,
    avgDuration: Math.round(
      logs.filter(l => typeof l.duration_ms === 'number').reduce((sum, l) => sum + (l.duration_ms || 0), 0) /
      Math.max(1, logs.filter(l => typeof l.duration_ms === 'number').length)
    ),
  };

  const sourceBreakdown = ['embed', 'admin', 'api_key', 'sso'].map(source => ({
    source,
    count: logs.filter(l => (l.source_type || 'admin') === source).length,
  })).filter(item => item.count > 0);

  const columns: ColumnsType<AuditLog> = [
    {
      title: '来源',
      dataIndex: 'source_type',
      key: 'source_type',
      width: 95,
      render: (v?: string) => <Tag color={getSourceColor(v)}>{v || 'admin'}</Tag>,
    },
    {
      title: '外部身份',
      key: 'external_identity',
      width: 210,
      ellipsis: true,
      render: (_, record) => record.external_system || record.external_user_id ? (
        <Space direction="vertical" size={0}>
          <Text strong>{record.external_system || '-'}</Text>
          <Text type="secondary" ellipsis style={{ maxWidth: 180 }}>{record.external_user_id || '-'}</Text>
        </Space>
      ) : '-',
    },
    {
      title: '接入应用',
      dataIndex: 'source_app_id',
      key: 'source_app_id',
      width: 160,
      ellipsis: true,
      render: (v?: string) => v || '-',
    },
    {
      title: 'EASP用户',
      key: 'easp_user',
      width: 180,
      ellipsis: true,
      render: (_, record) => record.user_uid || record.user_id || '-',
    },
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 100,
      render: (v: string) => <Tag color="blue">{v || '-'}</Tag>,
    },
    {
      title: '工具',
      dataIndex: 'tool',
      key: 'tool',
      width: 150,
      ellipsis: true,
      render: (v: string) => v || '-',
    },
    ...(!isMobile ? [
      {
        title: '资源',
        dataIndex: 'resource',
        key: 'resource',
        width: 120,
        ellipsis: true,
        render: (v?: string) => v || '-',
      },
      {
        title: '具体内容',
        dataIndex: 'detail',
        key: 'detail',
        ellipsis: true,
        render: (v?: string) => (
          <Text ellipsis style={{ maxWidth: 320 }} title={safeParseDetail(v)}>
            {safeParseDetail(v)}
          </Text>
        ),
      },
      {
        title: '结果',
        dataIndex: 'result',
        key: 'result',
        width: 95,
        render: (v?: string, record?: AuditLog) => <Tag color={getResultColor(v || record?.decision)}>{v || record?.decision || '-'}</Tag>,
      },
      {
        title: '耗时',
        dataIndex: 'duration_ms',
        key: 'duration_ms',
        width: 100,
        render: (v?: number) => v != null ? <Tag color={v > 3000 ? 'red' : v > 1000 ? 'orange' : 'green'}>{v} ms</Tag> : <Text type="secondary">-</Text>,
      },
    ] : []),
    {
      title: '时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 170,
      render: (v: string) => v ? new Date(v).toLocaleString() : '-',
    },
    {
      title: '详情',
      key: 'detail_action',
      width: 90,
      fixed: isMobile ? undefined : 'right',
      render: (_, record) => (
        <Button size="small" icon={<EyeOutlined />} onClick={() => setSelectedLog(record)}>
          查看
        </Button>
      ),
    },
  ];

  const detailText = safeParseDetail(selectedLog?.detail);

  return (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <div>
          <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><FileTextOutlined /> 审计可观测</Title>
          <Text type="secondary">追踪外部系统、外部用户、EASP内部用户、工具/Skill调用、授权决策、结果和耗时。</Text>
        </div>
        <Button icon={<ReloadOutlined />} onClick={() => load()} loading={loading}>刷新</Button>
      </Space>

      <Space direction="vertical" size="middle" style={{ width: '100%', marginBottom: 16 }}>
        <Alert
          type="info"
          showIcon
          message="权限主体仍是 EASP 内部用户；嵌入式访问会固化当次外部身份快照。"
          description="重点看三段链路：外部来源/应用/用户 → EASP 用户 → 工具或 Skill 的授权决策与执行结果。"
        />
        <div style={{ display: 'grid', gridTemplateColumns: isMobile ? '1fr' : 'repeat(5, minmax(0, 1fr))', gap: 12 }}>
          <Card><Statistic title="日志总数" value={summary.total} prefix={<FileTextOutlined />} /></Card>
          <Card><Statistic title="嵌入来源" value={summary.embed} prefix={<ApiOutlined />} /></Card>
          <Card><Statistic title="有外部身份" value={summary.externalIdentity} prefix={<UserOutlined />} /></Card>
          <Card><Statistic title="失败/拒绝" value={summary.failed} valueStyle={{ color: summary.failed ? '#cf1322' : undefined }} prefix={<WarningOutlined />} /></Card>
          <Card><Statistic title="平均耗时" value={summary.withDuration ? summary.avgDuration : 0} suffix="ms" prefix={<ClockCircleOutlined />} /></Card>
        </div>
        {sourceBreakdown.length > 0 && (
          <Card size="small" title="来源分布" extra={<Text type="secondary">基于当前查询结果</Text>}>
            <Space wrap>
              {sourceBreakdown.map(item => <Tag key={item.source} color={getSourceColor(item.source)}>{item.source}: {item.count}</Tag>)}
              <Tag icon={<CheckCircleOutlined />} color="green">成功/允许: {summary.total - summary.failed}</Tag>
            </Space>
          </Card>
        )}
      </Space>

      <Card size="small" style={{ marginBottom: 16 }} title="快速过滤" extra={<Text type="secondary">支持来源、外部身份、内部用户和工具定位</Text>}>
        <Form form={form} layout="inline" onFinish={load} style={{ rowGap: 8 }}>
          <Form.Item name="source_type" label="来源">
            <Select allowClear placeholder="全部" style={{ width: 130 }} options={[
              { label: '嵌入助手', value: 'embed' },
              { label: '管理后台', value: 'admin' },
              { label: 'API Key', value: 'api_key' },
              { label: 'SSO', value: 'sso' },
            ]} />
          </Form.Item>
          <Form.Item name="external_system" label="外部系统"><Input allowClear placeholder="crm" style={{ width: 140 }} /></Form.Item>
          <Form.Item name="external_user_id" label="外部用户"><Input allowClear placeholder="external_user_id" style={{ width: 180 }} /></Form.Item>
          <Form.Item name="source_app_id" label="App ID"><Input allowClear placeholder="app_xxx" style={{ width: 160 }} /></Form.Item>
          <Form.Item name="user_uid" label="EASP用户"><Input allowClear placeholder="usr_xxx" style={{ width: 160 }} /></Form.Item>
          <Form.Item name="tool" label="工具"><Input allowClear placeholder="tool" style={{ width: 140 }} /></Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />} loading={loading}>查询</Button>
            <Button onClick={() => { form.resetFields(); load({}); }}>重置</Button>
          </Space>
        </Form>
      </Card>

      <Table
        dataSource={logs}
        columns={columns}
        rowKey="id"
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={{ x: isMobile ? 980 : 1500 }}
        pagination={{ pageSize: isMobile ? 10 : 20, size: isMobile ? 'small' : undefined }}
      />

      <Drawer
        title="审计日志详情"
        open={!!selectedLog}
        onClose={() => setSelectedLog(null)}
        width={isMobile ? '100%' : 720}
      >
        {selectedLog && (
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Alert
              type={isFailure(selectedLog) ? 'error' : 'success'}
              showIcon
              message={isFailure(selectedLog) ? '本次调用失败或被拒绝' : '本次调用成功或被允许'}
              description="审计详情按 外部身份快照 → EASP权限主体 → 工具/Skill执行 → 请求上下文 展示。"
            />
            <Card size="small" title="身份链路">
              <Descriptions column={1} bordered size="small">
                <Descriptions.Item label="来源"><Tag color={getSourceColor(selectedLog.source_type)}>{selectedLog.source_type || 'admin'}</Tag></Descriptions.Item>
                <Descriptions.Item label="接入应用 App ID">{selectedLog.source_app_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="外部系统">{selectedLog.external_system || '-'}</Descriptions.Item>
                <Descriptions.Item label="外部用户ID">{selectedLog.external_user_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="EASP User UID">{selectedLog.user_uid || '-'}</Descriptions.Item>
                <Descriptions.Item label="EASP User ID">{selectedLog.user_id || '-'}</Descriptions.Item>
              </Descriptions>
            </Card>
            <Card size="small" title="工具 / Skill 执行">
              <Descriptions column={1} bordered size="small">
                <Descriptions.Item label="操作"><Tag color="blue">{selectedLog.action || '-'}</Tag></Descriptions.Item>
                <Descriptions.Item label="工具">{selectedLog.tool || '-'}</Descriptions.Item>
                <Descriptions.Item label="资源">{selectedLog.resource || '-'}</Descriptions.Item>
                <Descriptions.Item label="结果"><Tag color={getResultColor(selectedLog.result || selectedLog.decision)}>{selectedLog.result || '-'}</Tag></Descriptions.Item>
                <Descriptions.Item label="决策"><Tag color={getDecisionColor(selectedLog.decision)}>{selectedLog.decision || '-'}</Tag></Descriptions.Item>
                <Descriptions.Item label="耗时">{selectedLog.duration_ms != null ? `${selectedLog.duration_ms} ms` : '-'}</Descriptions.Item>
                <Descriptions.Item label="具体内容">
                  <Paragraph copyable style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>
                    {detailText}
                  </Paragraph>
                </Descriptions.Item>
              </Descriptions>
            </Card>
            <Card size="small" title="请求上下文">
              <Descriptions column={1} bordered size="small">
                <Descriptions.Item label="日志ID">{selectedLog.id}</Descriptions.Item>
                <Descriptions.Item label="Agent ID / Request ID">{selectedLog.agent_id || '-'}</Descriptions.Item>
                <Descriptions.Item label="IP">{selectedLog.ip || '-'}</Descriptions.Item>
                <Descriptions.Item label="User-Agent">{selectedLog.user_agent || '-'}</Descriptions.Item>
                <Descriptions.Item label="时间">{selectedLog.created_at ? new Date(selectedLog.created_at).toLocaleString() : '-'}</Descriptions.Item>
              </Descriptions>
            </Card>
          </Space>
        )}
      </Drawer>
    </div>
  );
};

export default AuditLogs;
