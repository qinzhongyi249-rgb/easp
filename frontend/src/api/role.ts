import client from './client';

export interface Role {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  tools?: string;
  allowed_mcp_tools?: string;
  allowed_skills?: string;
  rate_limit?: string;
  data_scope?: string;
  is_system?: boolean;
  is_default?: boolean;
  created_at: string;
  updated_at: string;
}

export const roleApi = {
  list: (tenantId: string) =>
    client.get(`/tenants/${tenantId}/roles`),
  listSystem: () =>
    client.get<Role[]>('/system/roles'),
  get: (tenantId: string, roleId: string) =>
    client.get<Role>(`/tenants/${tenantId}/roles/${roleId}`),
  create: (tenantId: string, data: Partial<Role>) =>
    client.post<Role>(`/tenants/${tenantId}/roles`, data),
  update: (tenantId: string, roleId: string, data: Partial<Role>) =>
    client.put<Role>(`/tenants/${tenantId}/roles/${roleId}`, data),
  delete: (tenantId: string, roleId: string) =>
    client.delete(`/tenants/${tenantId}/roles/${roleId}`),
  getUsers: (tenantId: string, roleId: string) =>
    client.get(`/tenants/${tenantId}/roles/${roleId}/users`),
};
