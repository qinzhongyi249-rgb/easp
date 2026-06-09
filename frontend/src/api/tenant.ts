import client from './client';

export interface Tenant {
  id: string;
  name: string;
  description: string;
  status: string;
  plan: string;
  expires_at: string | null;
  max_users: number;
  created_at: string;
  updated_at: string;
}

export const tenantApi = {
  list: () => client.get<Tenant[]>('/tenants'),
  get: (id: string) => client.get<Tenant>(`/tenants/${id}`),
  create: (data: Record<string, unknown>) => client.post<Tenant>('/tenants', data),
  update: (id: string, data: Partial<Tenant>) => client.put<Tenant>(`/tenants/${id}`, data),
  delete: (id: string) => client.delete(`/tenants/${id}`),
};
