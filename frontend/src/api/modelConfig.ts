import client from './client';

export interface ModelProvider {
  id: string;
  tenant_id: string;
  name: string;
  display_name?: string;
  type: string;
  base_url: string;
  api_key?: string;
  enabled?: boolean;
  created_at: string;
  updated_at: string;
}

export interface ModelConfig {
  id: string;
  tenant_id: string;
  provider_id: string;
  model_name: string;
  display_name?: string;
  temperature?: number;
  max_tokens?: number;
  is_default: boolean;
  enabled?: boolean;
  created_at: string;
  updated_at: string;
}

export const modelConfigApi = {
  // Providers
  listProviders: (tenantId: string) =>
    client.get<ModelProvider[]>(`/tenants/${tenantId}/model-providers`),
  getProvider: (tenantId: string, id: string) =>
    client.get<ModelProvider>(`/tenants/${tenantId}/model-providers/${id}`),
  createProvider: (tenantId: string, data: Partial<ModelProvider>) =>
    client.post<ModelProvider>(`/tenants/${tenantId}/model-providers`, data),
  updateProvider: (tenantId: string, id: string, data: Partial<ModelProvider>) =>
    client.put<ModelProvider>(`/tenants/${tenantId}/model-providers/${id}`, data),
  deleteProvider: (tenantId: string, id: string) =>
    client.delete(`/tenants/${tenantId}/model-providers/${id}`),

  // Configs
  listConfigs: (tenantId: string) =>
    client.get<ModelConfig[]>(`/tenants/${tenantId}/model-configs`),
  getConfig: (tenantId: string, id: string) =>
    client.get<ModelConfig>(`/tenants/${tenantId}/model-configs/${id}`),
  getDefaultConfig: (tenantId: string) =>
    client.get<ModelConfig>(`/tenants/${tenantId}/model-configs/default`),
  createConfig: (tenantId: string, data: Partial<ModelConfig>) =>
    client.post<ModelConfig>(`/tenants/${tenantId}/model-configs`, data),
  updateConfig: (tenantId: string, id: string, data: Partial<ModelConfig>) =>
    client.put<ModelConfig>(`/tenants/${tenantId}/model-configs/${id}`, data),
  deleteConfig: (tenantId: string, id: string) =>
    client.delete(`/tenants/${tenantId}/model-configs/${id}`),
  setDefault: (tenantId: string, id: string) =>
    client.put(`/tenants/${tenantId}/model-configs/${id}/default`),
};
