import client from './client';

export interface TenantUser {
  id: string;
  tenant_id: string;
  email: string;
  display_name: string;
  status: string;
  is_admin?: boolean;
  created_at: string;
  last_login_at?: string;
  login_count: number;
  deleted_at?: string | null;
}

export const userApi = {
  listByTenant: (tenantId: string) =>
    client.get<TenantUser[]>(`/tenants/${tenantId}/users`),
  get: (tenantId: string, userId: string) =>
    client.get<TenantUser>(`/tenants/${tenantId}/users/${userId}`),
  create: (tenantId: string, data: { email: string; password: string; display_name?: string }) =>
    client.post<TenantUser>(`/tenants/${tenantId}/users`, data),
  update: (tenantId: string, userId: string, data: Partial<TenantUser>) =>
    client.put<TenantUser>(`/tenants/${tenantId}/users/${userId}`, data),
  delete: (tenantId: string, userId: string) =>
    client.delete(`/tenants/${tenantId}/users/${userId}`),
  restore: (tenantId: string, userId: string) =>
    client.post(`/tenants/${tenantId}/users/${userId}/restore`),
  assignRole: (userId: string, roleId: string) =>
    client.post('/users/assign-role', { user_id: userId, role_id: roleId }),
  revokeRole: (userId: string, roleId: string) =>
    client.delete(`/users/${userId}/roles/${roleId}`),
  getRoles: (userId: string) =>
    client.get(`/users/${userId}/roles`),
  generateResetPassword: (tenantId: string, userId: string) =>
    client.post<{ password: string; saved: boolean }>(`/tenants/${tenantId}/users/${userId}/reset-password`),
  confirmResetPassword: (tenantId: string, userId: string, password: string) =>
    client.post<{ message: string; saved: boolean }>(`/tenants/${tenantId}/users/${userId}/reset-password`, { password }),
};
