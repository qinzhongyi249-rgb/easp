import React, { useState, useEffect } from 'react';
import { Table, Typography, App, Tag, Button, Drawer, Descriptions, Space } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { FileTextOutlined, EyeOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title, Text, Paragraph } = Typography;

interface LayoutContext { currentTenant: string; }

interface AuditLog {
  id: string;
  tenant_id: string;
  user_id?: string;
  agent_id?: string;
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

const safeParseDetail = (value?: string): string => {
  if (!value) return '-';
  let parsed: unknown = value;

  // detail 是 MySQL JSON 列，历史数据常见形态："\"调用MCP工具 xxx: {...}\""
  // 连续解析最多 3 次，把 JSON 字符串/对象还原成人能读的内容。
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
  try {
    return JSON.stringify(parsed, null, 2);
  } catch {
    return String(parsed);
  }
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

const AuditLogs: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedLog, setSelectedLog] = useState<AuditLog | null>(null);
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try {
      const res = await client.get(`/tenants/${currentTenant}/audit-logs`);
      setLogs(Array.isArray(res.data) ? res.data : []);
    } catch {
      message.error('加载审计日志失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const columns: ColumnsType<AuditLog> = [
    {
      title: '操作',
      dataIndex: 'action',
      key: 'action',
      width: 100,
      render: (v: string) => <Tag color="blue">{v || '-'}</Tag>,
    },
    {
      title: '工具/来源',
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
        width: 130,
        ellipsis: true,
        render: (v?: string) => v || '-',
      },
      {
        title: '具体内容',
        dataIndex: 'detail',
        key: 'detail',
        ellipsis: true,
        render: (v?: string) => (
          <Text ellipsis style={{ maxWidth: 360 }} title={safeParseDetail(v)}>
            {safeParseDetail(v)}
          </Text>
        ),
      },
      {
        title: '结果',
        dataIndex: 'result',
        key: 'result',
        width: 100,
        render: (v?: string) => <Tag color={getResultColor(v)}>{v || '-'}</Tag>,
      },
      {
        title: '决策',
        dataIndex: 'decision',
        key: 'decision',
        width: 100,
        render: (v?: string) => <Tag color={getDecisionColor(v)}>{v || '-'}</Tag>,
      },
      {
        title: '用户ID',
        dataIndex: 'user_id',
        key: 'user_id',
        width: 160,
        ellipsis: true,
        render: (v?: string) => v || '-',
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
        <Title level={isMobile ? 4 : 3} style={{ margin: 0 }}><FileTextOutlined /> 审计日志</Title>
        <Button onClick={load} loading={loading}>刷新</Button>
      </Space>
      <Table
        dataSource={logs}
        columns={columns}
        rowKey="id"
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={{ x: isMobile ? 760 : 1200 }}
        pagination={{ pageSize: isMobile ? 10 : 20, size: isMobile ? 'small' : undefined }}
      />

      <Drawer
        title="审计日志详情"
        open={!!selectedLog}
        onClose={() => setSelectedLog(null)}
        width={isMobile ? '100%' : 640}
      >
        {selectedLog && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="日志ID">{selectedLog.id}</Descriptions.Item>
            <Descriptions.Item label="操作"><Tag color="blue">{selectedLog.action || '-'}</Tag></Descriptions.Item>
            <Descriptions.Item label="工具/来源">{selectedLog.tool || '-'}</Descriptions.Item>
            <Descriptions.Item label="资源">{selectedLog.resource || '-'}</Descriptions.Item>
            <Descriptions.Item label="结果"><Tag color={getResultColor(selectedLog.result)}>{selectedLog.result || '-'}</Tag></Descriptions.Item>
            <Descriptions.Item label="决策"><Tag color={getDecisionColor(selectedLog.decision)}>{selectedLog.decision || '-'}</Tag></Descriptions.Item>
            <Descriptions.Item label="用户ID">{selectedLog.user_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="Agent ID">{selectedLog.agent_id || '-'}</Descriptions.Item>
            <Descriptions.Item label="耗时">{selectedLog.duration_ms != null ? `${selectedLog.duration_ms} ms` : '-'}</Descriptions.Item>
            <Descriptions.Item label="IP">{selectedLog.ip || '-'}</Descriptions.Item>
            <Descriptions.Item label="User-Agent">{selectedLog.user_agent || '-'}</Descriptions.Item>
            <Descriptions.Item label="时间">{selectedLog.created_at ? new Date(selectedLog.created_at).toLocaleString() : '-'}</Descriptions.Item>
            <Descriptions.Item label="具体内容">
              <Paragraph copyable style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>
                {detailText}
              </Paragraph>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Drawer>
    </div>
  );
};

export default AuditLogs;
