import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import type { User } from '../api/auth';
import { authApi } from '../api/auth';

interface AuthContextType {
  user: User | null;
  loading: boolean;
  login: (identifier: string, password: string, tenantId?: string) => Promise<void>;
  logout: () => void;
  reloadUser: () => Promise<void>;
  isAuthenticated: boolean;
  isAdmin: boolean;
  tools: string[];
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const loadUser = useCallback(async () => {
    const token = localStorage.getItem('access_token');
    if (!token) {
      setLoading(false);
      return;
    }
    try {
      const res = await authApi.getMe();
      setUser(res.data);
    } catch {
      localStorage.removeItem('access_token');
      localStorage.removeItem('refresh_token');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadUser();
  }, [loadUser]);

  const login = async (identifier: string, password: string, tenantId?: string) => {
    const res = await authApi.login({ account: identifier, password, tenant_id: tenantId });
    const data = res.data as { user: User; tokens: { access_token: string; refresh_token: string } };
    localStorage.setItem('access_token', data.tokens.access_token);
    localStorage.setItem('refresh_token', data.tokens.refresh_token);
    setUser(data.user);
  };

  const logout = () => {
    localStorage.removeItem('access_token');
    localStorage.removeItem('refresh_token');
    setUser(null);
    window.location.href = '/login';
  };

  // 从API返回的user对象中读取is_admin
  const isAdmin = user?.is_admin || false;
  const tools = user?.tools || [];

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, reloadUser: loadUser, isAuthenticated: !!user, isAdmin, tools }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
};
