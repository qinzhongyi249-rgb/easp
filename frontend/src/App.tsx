import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, App as AntdApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import MainLayout from './layouts/MainLayout';
import Login from './pages/Login';
import TenantLogin from './pages/TenantLogin';
import Dashboard from './pages/Dashboard';
import Tenants from './pages/Tenants';
import Users from './pages/Users';
import Roles from './pages/Roles';
import Connectors from './pages/Connectors';
import MCPTools from './pages/MCPTools';
import Skills from './pages/Skills';
import Memory from './pages/Memory';
import ModelConfig from './pages/ModelConfig';
import AuditLogs from './pages/AuditLogs';
import SSOConfig from './pages/SSOConfig';
import Assistant from './pages/Assistant';

const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated, loading } = useAuth();
  if (loading) return null;
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />;
};

const AppRoutes = () => {
  const { isAuthenticated, loading } = useAuth();
  if (loading) return null;

  return (
    <Routes>
      <Route path="/login" element={isAuthenticated ? <Navigate to="/dashboard" replace /> : <Login />} />
      <Route path="/sso/:tenantId" element={<TenantLogin />} />
      <Route path="/" element={<ProtectedRoute><MainLayout /></ProtectedRoute>}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<Dashboard />} />
        <Route path="tenants" element={<Tenants />} />
        <Route path="users" element={<Users />} />
        <Route path="roles" element={<Roles />} />
        <Route path="connectors" element={<Connectors />} />
        <Route path="mcp-tools" element={<MCPTools />} />
        <Route path="skills" element={<Skills />} />
        <Route path="memory" element={<Memory />} />
        <Route path="model-config" element={<ModelConfig />} />
        <Route path="audit-logs" element={<AuditLogs />} />
        <Route path="sso-config" element={<SSOConfig />} />
        <Route path="assistant" element={<Assistant />} />
      </Route>
      <Route path="*" element={<Navigate to="/dashboard" replace />} />
    </Routes>
  );
};

function App() {
  return (
    <ConfigProvider locale={zhCN} theme={{ token: { colorPrimary: '#1677ff' } }}>
      <AntdApp>
        <BrowserRouter>
          <AuthProvider>
            <AppRoutes />
          </AuthProvider>
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  );
}

export default App;
