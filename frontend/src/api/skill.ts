import client from './client';

export interface Skill {
  id: string;
  tenant_id: string;
  name: string;
  description: string;
  steps?: string;
  triggers?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface SkillExecution {
  id: string;
  skill_id: string;
  tenant_id: string;
  status: string;
  inputs?: string;
  outputs?: string;
  started_at: string;
  completed_at?: string;
}

export const skillApi = {
  list: (tenantId: string) =>
    client.get<Skill[]>(`/tenants/${tenantId}/skills`),
  get: (tenantId: string, id: string) =>
    client.get<Skill>(`/tenants/${tenantId}/skills/${id}`),
  create: (tenantId: string, data: Partial<Skill>) =>
    client.post<Skill>(`/tenants/${tenantId}/skills`, data),
  update: (tenantId: string, id: string, data: Partial<Skill>) =>
    client.put<Skill>(`/tenants/${tenantId}/skills/${id}`, data),
  delete: (tenantId: string, id: string) =>
    client.delete(`/tenants/${tenantId}/skills/${id}`),
  execute: (tenantId: string, skillId: string, inputs: Record<string, unknown>) =>
    client.post(`/tenants/${tenantId}/skills/${skillId}/execute`, { inputs }),
  listExecutions: (tenantId: string, skillId?: string) =>
    client.get<{ executions: SkillExecution[] }>(`/tenants/${tenantId}/skill-executions`, { params: { skill_id: skillId } }),
};
