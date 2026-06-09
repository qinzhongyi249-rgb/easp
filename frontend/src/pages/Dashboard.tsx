import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Typography, Spin } from 'antd';
import {
  TeamOutlined,
  UserOutlined,
  ApiOutlined,
  ToolOutlined,
  BulbOutlined,
} from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import type { TenantUser } from '../api/user';
import type { Role } from '../api/role';
import type { Connector } from '../api/connector';
import type { MCPTool } from '../api/mcpTool';
import type { Skill } from '../api/skill';
import { tenantApi } from '../api/tenant';
import { userApi } from '../api/user';
import { roleApi } from '../api/role';
import { connectorApi } from '../api/connector';
import { mcpToolApi } from '../api/mcpTool';
import { skillApi } from '../api/skill';

const { Title } = Typography;

interface LayoutContext {
  currentTenant: string;
}

const Dashboard: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { isAdmin } = useAuth();
  const [tenantCount, setTenantCount] = useState(0);
  const [users, setUsers] = useState<TenantUser[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [connectors, setConnectors] = useState<Connector[]>([]);
  const [tools, setTools] = useState<MCPTool[]>([]);
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      try {
        // 只有超级管理员才加载租户数
        if (isAdmin) {
          const tenantRes = await tenantApi.list();
          setTenantCount((tenantRes.data || []).length);
        }

        if (currentTenant) {
          const [u, r, c, t, s] = await Promise.allSettled([
            userApi.listByTenant(currentTenant),
            roleApi.list(currentTenant),
            connectorApi.list(currentTenant),
            mcpToolApi.list(currentTenant),
            skillApi.list(currentTenant),
          ]);
          if (u.status === 'fulfilled') setUsers(u.value.data || []);
          if (r.status === 'fulfilled') setRoles(r.value.data || []);
          if (c.status === 'fulfilled') setConnectors(c.value.data || []);
          if (t.status === 'fulfilled') setTools(t.value.data || []);
          if (s.status === 'fulfilled') setSkills(s.value.data || []);
        }
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    };
    load();
  }, [currentTenant, isAdmin]);

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />;

  return (
    <div>
      <Title level={3}>仪表盘</Title>
      <Row gutter={[24, 24]}>
        {/* 只有超级管理员才显示租户数 */}
        {isAdmin && (
          <Col xs={24} sm={12} lg={8}>
            <Card hoverable>
              <Statistic title="租户数" value={tenantCount} prefix={<TeamOutlined />} />
            </Card>
          </Col>
        )}
        <Col xs={24} sm={12} lg={8}>
          <Card hoverable>
            <Statistic title="用户数" value={users.length} prefix={<UserOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card hoverable>
            <Statistic title="角色数" value={roles.length} prefix={<TeamOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card hoverable>
            <Statistic title="连接器" value={connectors.length} prefix={<ApiOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card hoverable>
            <Statistic title="MCP工具" value={tools.length} prefix={<ToolOutlined />} />
          </Card>
        </Col>
        <Col xs={24} sm={12} lg={8}>
          <Card hoverable>
            <Statistic title="技能" value={skills.length} prefix={<BulbOutlined />} />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
