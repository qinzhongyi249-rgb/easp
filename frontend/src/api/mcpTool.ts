import client from './client';

export interface MCPTool {
  id: string;
  tenant_id: string;
  connector_id: string;
  name: string;
  description: string;
  method: string;
  path: string;
  parameters?: string;
  input_schema?: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export const mcpToolApi = {
  list: (tenantId: string) =>
    client.get<MCPTool[]>(`/tenants/${tenantId}/mcp-tools`),
  get: (tenantId: string, id: string) =>
    client.get<MCPTool>(`/tenants/${tenantId}/mcp-tools/${id}`),
  create: (tenantId: string, data: Partial<MCPTool>) =>
    client.post<MCPTool>(`/tenants/${tenantId}/mcp-tools`, data),
  update: (tenantId: string, id: string, data: Partial<MCPTool>) =>
    client.put<MCPTool>(`/tenants/${tenantId}/mcp-tools/${id}`, data),
  delete: (tenantId: string, id: string) =>
    client.delete(`/tenants/${tenantId}/mcp-tools/${id}`),
  toggleEnabled: (tenantId: string, id: string, enabled: boolean) =>
    client.put(`/tenants/${tenantId}/mcp-tools/${id}/enabled`, { enabled }),
};
