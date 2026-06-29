import React, { useState, useEffect } from 'react';
import { Layout, Menu, Avatar, Dropdown, Spin, Select, Drawer, Button } from 'antd';
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
  BarChartOutlined,
  MenuOutlined,
} from '@ant-design/icons';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import type { Tenant } from '../api/tenant';
import { tenantApi } from '../api/tenant';
import FloatingAssistant from '../components/FloatingAssistant';
import { FEATURE_MENU_PERMISSIONS } from '../config/menuPermissions';

const { Header, Sider, Content } = Layout;

// 移动端检测 hook
const useIsMobile = () => {
  const [isMobile, setIsMobile] = useState(window.innerWidth < 768);
  useEffect(() => {
    const handleResize = () => setIsMobile(window.innerWidth < 768);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);
  return isMobile;
};

const MainLayout: React.FC = () => {
  const { user, logout, loading, isAdmin, tools } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const isMobile = useIsMobile();
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [tenants, setTenants] = useState<Tenant[]>([]);
  const [currentTenant, setCurrentTenant] = useState<string>('');

  useEffect(() => {
    const userTenantId = user?.tenant_id;
    tenantApi.list().then((res) => {
      const tenantList = res.data || [];
      setTenants(tenantList);
      if (tenantList.length > 0 && !currentTenant) {
        const found = userTenantId ? tenantList.find((t) => t.id === userTenantId) : null;
        setCurrentTenant(found ? userTenantId! : tenantList[0].id);
      }
    }).catch(() => {});
  }, []);

  // 移动端自动收起侧边栏
  useEffect(() => {
    if (isMobile) setCollapsed(true);
  }, [isMobile]);

  if (loading) {
    return <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}><Spin size="large" /></div>;
  }

  if (!user) {
    window.location.href = '/login';
    return null;
  }

  const hasTool = (tool: string) => isAdmin || tools.includes('*') || tools.includes(tool);

  const menuIconMap: Record<string, React.ReactNode> = {
    users: <UserOutlined />,
    roles: <SafetyCertificateOutlined />,
    connectors: <ApiOutlined />,
    'mcp-tools': <ToolOutlined />,
    skills: <BulbOutlined />,
    memory: <DatabaseOutlined />,
    'model-config': <SettingOutlined />,
    'sso-config': <KeyOutlined />,
    assistant: <RobotOutlined />,
    'usage-analytics': <BarChartOutlined />,
    'audit-logs': <FileTextOutlined />,
    'api-keys': <KeyOutlined />,
  };

  const menuItems = [
    { key: '/dashboard', icon: <DashboardOutlined />, label: '仪表盘' },
    ...(isAdmin ? [{ key: '/tenants', icon: <TeamOutlined />, label: '租户管理' }] : []),
    ...FEATURE_MENU_PERMISSIONS
      .filter(item => hasTool(item.value))
      .map(item => ({ key: `/${item.value}`, icon: menuIconMap[item.value], label: item.label })),
  ];

  const userMenuItems = [
    { key: 'profile', label: `${user.email}` },
    { type: 'divider' as const },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: logout },
  ];

  const handleMenuClick = ({ key }: { key: string }) => {
    navigate(key);
    if (isMobile) setDrawerOpen(false);
  };

  // 侧边栏内容
  const siderContent = (
    <>
      <div style={{ height: 64, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: collapsed && !isMobile ? 16 : 20, fontWeight: 'bold', letterSpacing: 2 }}>
        {collapsed && !isMobile ? 'E' : 'EASP Platform'}
      </div>
      <Menu
        theme="dark"
        mode="inline"
        selectedKeys={[location.pathname]}
        items={menuItems}
        onClick={handleMenuClick}
      />
    </>
  );

  return (
    <Layout style={{ minHeight: '100vh' }}>
      {/* 桌面端侧边栏 */}
      {!isMobile && (
        <Sider trigger={null} collapsible collapsed={collapsed} theme="dark" width={220}>
          {siderContent}
        </Sider>
      )}

      {/* 移动端抽屉侧边栏 */}
      {isMobile && (
        <Drawer
          placement="left"
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          width={220}
          styles={{ body: { padding: 0, background: '#001529' } }}
          closable={false}
        >
          {siderContent}
        </Drawer>
      )}

      <Layout>
        <Header style={{
          background: '#fff',
          padding: isMobile ? '0 12px' : '0 24px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
          height: isMobile ? 56 : 64,
          position: 'sticky',
          top: 0,
          zIndex: 100,
        }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: isMobile ? 8 : 16 }}>
            {isMobile ? (
              <Button type="text" icon={<MenuOutlined />} onClick={() => setDrawerOpen(true)} style={{ fontSize: 18 }} />
            ) : (
              React.createElement(collapsed ? MenuUnfoldOutlined : MenuFoldOutlined, {
                onClick: () => setCollapsed(!collapsed),
                style: { fontSize: 18, cursor: 'pointer' },
              })
            )}
            {tenants.length > 0 && (
              <Select
                value={currentTenant}
                onChange={(v) => {
                  setCurrentTenant(v);
                  window.dispatchEvent(new CustomEvent('tenant-change', { detail: v }));
                }}
                style={{ width: isMobile ? 120 : 200 }}
                placeholder="选择租户"
                options={tenants.map((t) => ({ value: t.id, label: t.name }))}
                size={isMobile ? 'small' : 'middle'}
              />
            )}
          </div>
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: isMobile ? 4 : 8 }}>
              <Avatar icon={<UserOutlined />} size={isMobile ? 'small' : 'default'} />
              {!isMobile && <span>{user.display_name || user.email}</span>}
            </div>
          </Dropdown>
        </Header>

        <Content style={{
          margin: isMobile ? 8 : 24,
          padding: isMobile ? 12 : 24,
          background: '#fff',
          borderRadius: 8,
          minHeight: 280,
        }}>
          <Outlet context={{ currentTenant }} />
        </Content>
      </Layout>

      {currentTenant && <FloatingAssistant tenantId={currentTenant} userId={user?.id} />}
    </Layout>
  );
};

export default MainLayout;
