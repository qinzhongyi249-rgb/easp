import React, { Suspense, lazy } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ConfigProvider, App as AntdApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import MainLayout from './layouts/MainLayout';

const Home = lazy(() => import('./pages/Home'));
const Login = lazy(() => import('./pages/Login'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Tenants = lazy(() => import('./pages/Tenants'));
const Users = lazy(() => import('./pages/Users'));
const Roles = lazy(() => import('./pages/Roles'));
const Connectors = lazy(() => import('./pages/Connectors'));
const MCPTools = lazy(() => import('./pages/MCPTools'));
const Skills = lazy(() => import('./pages/Skills'));
const Memory = lazy(() => import('./pages/Memory'));
const ModelConfig = lazy(() => import('./pages/ModelConfig'));
const AuditLogs = lazy(() => import('./pages/AuditLogs'));
const APIKeys = lazy(() => import('./pages/APIKeys'));
const ApiKeyAccessDoc = lazy(() => import('./pages/ApiKeyAccessDoc'));
const SSOConfig = lazy(() => import('./pages/SSOConfig'));
const Assistant = lazy(() => import('./pages/Assistant'));
const UsageAnalytics = lazy(() => import('./pages/UsageAnalytics'));

const ProtectedRoute: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { isAuthenticated, loading } = useAuth();
  if (loading) return null;
  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />;
};

const AppRoutes = () => {
  const { isAuthenticated, loading } = useAuth();
  if (loading) return null;

  return (
    <Suspense fallback={null}>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/login" element={isAuthenticated ? <Navigate to="/dashboard" replace /> : <Login />} />
        <Route path="/sso/:tenantId" element={isAuthenticated ? <Navigate to="/dashboard" replace /> : <Login />} />
        <Route path="/docs/api-key-access" element={<ApiKeyAccessDoc />} />
        <Route path="/admin" element={<ProtectedRoute><MainLayout /></ProtectedRoute>}>
          <Route index element={<Navigate to="/admin/dashboard" replace />} />
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
          <Route path="api-keys" element={<APIKeys />} />
          <Route path="sso-config" element={<SSOConfig />} />
          <Route path="assistant" element={<Assistant />} />
          <Route path="usage-analytics" element={<UsageAnalytics />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
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
