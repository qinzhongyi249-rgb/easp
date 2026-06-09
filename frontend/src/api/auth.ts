import client from './client';

export interface LoginParams {
  email: string;
  password: string;
}

export interface RegisterParams {
  tenant_id: string;
  email: string;
  password: string;
  display_name?: string;
}

export interface User {
  id: string;
  tenant_id: string;
  email: string;
  display_name: string;
  status: string;
  is_admin?: boolean;
  role_names?: string[];
  tools?: string[];
  created_at: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
  expires_at: string;
}

export interface AuthResponse {
  user: User;
  tokens: TokenPair;
}

export const authApi = {
  login: (params: LoginParams) =>
    client.post<AuthResponse>('/auth/login', params),
  register: (params: RegisterParams) =>
    client.post<AuthResponse>('/auth/register', params),
  refreshToken: (refresh_token: string) =>
    client.post<TokenPair>('/auth/refresh', { refresh_token }),
  getMe: () => client.get<User>('/me'),
  changePassword: (old_password: string, new_password: string) =>
    client.put('/me/password', { old_password, new_password }),
  getPermissions: () => client.get<{ permissions: string[] }>('/me/permissions'),
};
