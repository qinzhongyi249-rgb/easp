import client from './client';

export interface Connector {
  id: string;
  tenant_id: string;
  name: string;
  type: string;
  base_url: string;
  transport_type?: 'sse' | 'streamable_http';
  mcp_server_url?: string;
  headers?: string; // JSON: 自定义HTTP头
  auth_type?: string;
  auth_config?: string;
  credential_mode?: 'static' | 'user_token' | 'none';
  user_token_header?: string;
  user_token_prefix?: string;
  user_token_required_sso?: boolean;
  openapi_spec?: string;
  status: string;
  tools_count?: number;
  created_at: string;
  updated_at: string;
}

export const connectorApi = {
  list: (tenantId: string) =>
    client.get<Connector[]>(`/tenants/${tenantId}/connectors`),
  get: (tenantId: string, id: string) =>
    client.get<Connector>(`/tenants/${tenantId}/connectors/${id}`),
  create: (tenantId: string, data: Partial<Connector>) =>
    client.post<Connector>(`/tenants/${tenantId}/connectors`, data),
  update: (tenantId: string, id: string, data: Partial<Connector>) =>
    client.put<Connector>(`/tenants/${tenantId}/connectors/${id}`, data),
  delete: (tenantId: string, id: string) =>
    client.delete(`/tenants/${tenantId}/connectors/${id}`),
  syncOpenAPI: (tenantId: string, connectorId: string) =>
    client.post(`/tenants/${tenantId}/connectors/${connectorId}/sync`),
  getOpenAPISpec: (tenantId: string, connectorId: string) =>
    client.get(`/tenants/${tenantId}/connectors/${connectorId}/openapi`),
  updateOpenAPISpec: (tenantId: string, connectorId: string, spec: string) =>
    client.put(`/tenants/${tenantId}/connectors/${connectorId}/openapi`, { spec }),
};
