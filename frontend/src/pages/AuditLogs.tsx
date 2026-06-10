import React, { useState, useEffect } from 'react';
import { Table, Typography, App } from 'antd';
import { FileTextOutlined } from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import client from '../api/client';

const { Title } = Typography;
interface LayoutContext { currentTenant: string; }

interface AuditLog {
  id: string;
  action: string;
  resource_type: string;
  resource_id: string;
  user_email: string;
  created_at: string;
}

const AuditLogs: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { message } = App.useApp();
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(false);
  const isMobile = window.innerWidth < 768;

  const load = async () => {
    if (!currentTenant) return;
    setLoading(true);
    try { 
      const res = await client.get(`/tenants/${currentTenant}/audit-logs`); 
      setLogs(res.data || []); 
    }
    catch { message.error('加载失败'); }
    finally { setLoading(false); }
  };

  useEffect(() => { load(); }, [currentTenant]);

  const columns = [
    { title: '操作', dataIndex: 'action', key: 'action' },
    { title: '资源', dataIndex: 'resource_type', key: 'resource_type' },
    ...(!isMobile ? [
      { title: '资源ID', dataIndex: 'resource_id', key: 'resource_id', ellipsis: true },
      { title: '用户', dataIndex: 'user_email', key: 'user_email', ellipsis: true },
    ] : []),
    { title: '时间', dataIndex: 'created_at', key: 'created_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
  ];

  return (
    <div>
      <Title level={isMobile ? 4 : 3}><FileTextOutlined /> 审计日志</Title>
      <Table 
        dataSource={logs} 
        columns={columns} 
        rowKey="id" 
        loading={loading}
        size={isMobile ? 'small' : 'middle'}
        scroll={isMobile ? { x: 400 } : undefined}
        pagination={isMobile ? { pageSize: 10, size: 'small' } : undefined}
      />
    </div>
  );
};

export default AuditLogs;
