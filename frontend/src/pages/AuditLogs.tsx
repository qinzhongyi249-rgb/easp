import React, { useState, useEffect } from 'react';
import { Table, Typography, Tag, App } from 'antd';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title } = Typography;
interface LayoutContext { currentTenant: string; }

interface AuditLog {
  id: string;
  tenant_id: string;
  user_id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

const AuditLogs: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [data, setData] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { const res = await client.get(`/tenants/${currentTenant}/audit-logs`); setData(Array.isArray(res.data) ? res.data : []); }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const columns = [
    { title: '操作', dataIndex: 'action', key: 'action', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { title: '资源类型', dataIndex: 'resource_type', key: 'resource_type' },
    { title: '资源ID', dataIndex: 'resource_id', key: 'resource_id', ellipsis: true },
    { title: '用户ID', dataIndex: 'user_id', key: 'user_id', ellipsis: true },
    { title: 'IP', dataIndex: 'ip_address', key: 'ip_address' },
    { title: '详情', dataIndex: 'details', key: 'details', ellipsis: true },
    { title: '时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
  ];

  return (
    <div>
      <Title level={3}>审计日志</Title>
      <Table dataSource={data} columns={columns} rowKey="id" loading={loading} />
    </div>
  );
};

export default AuditLogs;
