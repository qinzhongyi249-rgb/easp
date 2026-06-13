import client from './client';

export interface APIKey {
  id: string;
  tenant_id: string;
  user_id: string;
  user_email?: string;
  user_display_name?: string;
  name: string;
  key?: string;        // 只在创建时返回
  key_prefix: string;
  scopes?: string[] | string | null;
  enabled: boolean;
  expires_at: string | null;
  last_used_at: string | null;
  usage_count: number;
  created_at: string;
}

export const apiKeyApi = {
  list: (tenantId: string) =>
    client.get<APIKey[]>(`/tenants/${tenantId}/api-keys`),
  create: (tenantId: string, data: { name: string; scopes?: string[]; expires_in?: number }) =>
    client.post<APIKey>(`/tenants/${tenantId}/api-keys`, data),
  delete: (tenantId: string, keyId: string) =>
    client.delete(`/tenants/${tenantId}/api-keys/${keyId}`),
  toggle: (tenantId: string, keyId: string, enabled: boolean) =>
    client.put(`/tenants/${tenantId}/api-keys/${keyId}/toggle`, { enabled }),
};
