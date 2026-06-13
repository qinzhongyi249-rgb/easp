import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Typography, Spin } from 'antd';
import {
  TeamOutlined,
  UserOutlined,
  ApiOutlined,
  ToolOutlined,
  BulbOutlined,
  ThunderboltOutlined,
  RobotOutlined,
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
import { usageApi } from '../api/usage';
import type { UsageSummary } from '../api/usage';

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
  const [usage, setUsage] = useState<UsageSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const isMobile = window.innerWidth < 768;

  useEffect(() => {
    const load = async () => {
      setLoading(true);
      try {
        if (isAdmin) {
          const tenantRes = await tenantApi.list();
          setTenantCount((tenantRes.data || []).length);
        }

        if (currentTenant) {
          const [u, r, c, t, s, usageRes] = await Promise.allSettled([
            userApi.listByTenant(currentTenant),
            roleApi.list(currentTenant),
            connectorApi.list(currentTenant),
            mcpToolApi.list(currentTenant),
            skillApi.list(currentTenant),
            usageApi.summary(currentTenant),
          ]);
          if (u.status === 'fulfilled') setUsers(u.value.data || []);
          if (r.status === 'fulfilled') setRoles(r.value.data || []);
          if (c.status === 'fulfilled') setConnectors(c.value.data || []);
          if (t.status === 'fulfilled') setTools(t.value.data || []);
          if (s.status === 'fulfilled') setSkills(s.value.data || []);
          if (usageRes.status === 'fulfilled') setUsage(usageRes.value.data);
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
      <Title level={isMobile ? 4 : 3} style={{ marginBottom: isMobile ? 16 : 24 }}>仪表盘</Title>
      <Row gutter={[isMobile ? 12 : 24, isMobile ? 12 : 24]}>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="今日Tokens" value={usage?.today_tokens || 0} prefix={<ThunderboltOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="今日模型调用" value={usage?.today_model_calls || 0} prefix={<RobotOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="今日工具调用" value={usage?.today_tool_calls || 0} prefix={<ToolOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        {isAdmin && (
          <Col xs={12} sm={12} lg={8}>
            <Card hoverable size={isMobile ? 'small' : 'default'}>
              <Statistic title="租户数" value={tenantCount} prefix={<TeamOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
            </Card>
          </Col>
        )}
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="用户数" value={users.length} prefix={<UserOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="角色数" value={roles.length} prefix={<TeamOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="连接器" value={connectors.length} prefix={<ApiOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="MCP工具" value={tools.length} prefix={<ToolOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
        <Col xs={12} sm={12} lg={8}>
          <Card hoverable size={isMobile ? 'small' : 'default'}>
            <Statistic title="技能" value={skills.length} prefix={<BulbOutlined />} valueStyle={{ fontSize: isMobile ? 20 : 24 }} />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
