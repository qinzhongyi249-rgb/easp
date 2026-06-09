import React, { useState, useEffect } from 'react';
import { Layout, Menu, Avatar, Dropdown, Spin, Select } from 'antd';
import {
  DashboardOutlined,
  TeamOutlined,
  UserOutlined,
  SafetyCertificateOutlined,
  ApiOutlined,
  ToolOutlined,
  BulbOutlined,
  DatabaseOutlined,
  SettingOutlined,
  FileTextOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  KeyOutlined,
  RobotOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import type { Tenant } from '../api/tenant';
import { tenantApi } from '../api/tenant';
import FloatingAssistant from '../components/FloatingAssistant';

const { Header, Sider, Content } = Layout;

const MainLayout: React.FC = () => {
  const { user, logout, loading, isAdmin, tools } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(false);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [currentTenant, setCurrentTenant] = useState<string>('');

  useEffect(() => {
    // 从JWT中获取用户的tenant_id
    const userTenantId = user?.tenant_id;
    tenantApi.list().then((res) => {
      const tenantList = res.data || [];
      setTenants(tenantList);
      if (tenantList.length > 0 && !currentTenant) {
        // 优先使用用户自己的tenant_id，不存在则用第一个
        const found = userTenantId ? tenantList.find((t) => t.id === userTenantId) : null;
        setCurrentTenant(found ? userTenantId! : tenantList[0].id);
      }
    }).catch(() => {});
  }, []);

  if (loading) {
    return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}><Spin size="large" /></div>;
  }

  if (!user) {
    window.location.href = '/login';
    return null;
  }

  // 判断是否有某个工具权限（"*" 通配符表示全部权限）
  const hasTool = (tool: string) => isAdmin || tools.includes('*') || tools.includes(tool);

  const menuItems = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
    ...(hasTool('users') ? [
      { key: '/users', icon: <UserOutlined />, label: '用户管理' },
    ] : []),
    ...(hasTool('roles') ? [
      { key: '/roles', icon: <SafetyCertificateOutlined />, label: '角色管理' },
    ] : []),
    ...(isAdmin ? [
      { key: '/tenants', icon: <TeamOutlined />, label: '租户管理' },
    ] : []),
    ...(hasTool('connectors') ? [
      { key: '/connectors', icon: <ApiOutlined />, label: '连接器' },
    ] : []),
    ...(hasTool('mcp-tools') ? [
      { key: '/mcp-tools', icon: <ToolOutlined />, label: 'MCP工具' },
    ] : []),
    ...(hasTool('skills') ? [
      { key: '/skills', icon: <BulbOutlined />, label: '技能管理' },
    ] : []),
    ...(hasTool('memory') ? [
      { key: '/memory', icon: <DatabaseOutlined />, label: '记忆管理' },
    ] : []),
    ...(hasTool('model-config') ? [
      { key: '/model-config', icon: <SettingOutlined />, label: '模型配置' },
    ] : []),
    ...(hasTool('sso-config') ? [
      { key: '/sso-config', icon: <KeyOutlined />, label: 'SSO配置' },
    ] : []),
    { key: '/assistant', icon: <RobotOutlined />, label: 'AI 助手' },
    ...(hasTool('audit-logs') ? [
      { key: '/audit-logs', icon: <FileTextOutlined />, label: '审计日志' },
    ] : []),
  ];

  const userMenuItems = [
    { key: 'profile', label: `${user.email}` },
    { type: 'divider' as const },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: logout },
  ];

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider trigger={null} collapsible collapsed={collapsed} theme="dark" width={220}>
        <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: collapsed ? 16 : 20, fontWeight: 'bold', letterSpacing: 2 }}>
          {collapsed ? 'E' : 'EASP Platform'}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[location.pathname]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', boxShadow: '0 1px 4px rgba(0,0,0,0.08)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
            {React.createElement(collapsed ? MenuUnfoldOutlined : MenuFoldOutlined, {
              onClick: () => setCollapsed(!collapsed),
              style: { fontSize: 18, cursor: 'pointer' },
            })}
            {tenants.length > 0 && (
              <Select
                value={currentTenant}
                onChange={(v) => {
                  setCurrentTenant(v);
                  window.dispatchEvent(new CustomEvent('tenant-change', { detail: v }));
                }}
                style={{ width: 200 }}
                placeholder="选择租户"
                options={tenants.map((t) => ({ value: t.id, label: t.name }))}
              />
            )}
          </div>
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8 }}>
              <Avatar icon={<UserOutlined />} />
              <span>{user.display_name || user.email}</span>
            </div>
          </Dropdown>
        </Header>
        <Content style={{ margin: 24, padding: 24, background: '#fff', borderRadius: 8, minHeight: 280 }}>
          <Outlet context={{ currentTenant }} />
        </Content>
      </Layout>
      {/* 全局浮动AI助手 */}
      {currentTenant && <FloatingAssistant tenantId={currentTenant} />}
    </Layout>
  );
};

export default MainLayout;
