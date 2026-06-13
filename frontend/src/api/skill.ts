import client from './client';

// Skill 步骤定义
export interface SkillStep {
  name: string;
  type: string; // mcp_tool / condition / assign / http_request
  action?: string;
  params?: Record<string, unknown>;
  condition?: string;
  next_on_ok?: string;
  next_on_fail?: string;
  output_var?: string;
  description?: string;
}

// JSON Schema 定义
export interface JsonSchema {
  type: string;
  properties?: Record<string, {
    type: string;
    title?: string;
    description?: string;
    default?: unknown;
    enum?: unknown[];
    required?: boolean;
  }>;
  required?: string[];
}

export interface Skill {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  category?: string;
  version: string;
  tags?: string; // JSON array
  triggers?: string; // JSON
  input_schema?: string; // JSON Schema
  output_schema?: string; // JSON Schema
  steps: string; // JSON array of SkillStep
  permission_topology?: string;
  status: string; // draft/active/archived
  usage_count: number;
  last_used_at?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface StepResult {
  step_name: string;
  status: string; // completed/success/failed/skipped
  outputs?: Record<string, unknown>;
  error?: string;
  duration_ms: number;
}

export interface SkillExecution {
  id: string;
  skill_id: string;
  tenant_id: string;
  status: string; // running/completed/success/failed
  inputs?: string | Record<string, unknown>; // JSON or native object
  outputs?: string | Record<string, unknown>; // JSON or native object
  step_results?: string | StepResult[]; // JSON array or native array
  started_at: string;
  ended_at?: string;
  error?: string;
  duration_ms?: number;
}

// 技能分类
export const SKILL_CATEGORIES = [
  { label: '数据处理', value: 'data_process' },
  { label: '工作流', value: 'workflow' },
  { label: 'API调用', value: 'api_call' },
  { label: 'MCP工具', value: 'mcp_tool' },
  { label: '自定义', value: 'custom' },
];

// 步骤类型
export const STEP_TYPES = [
  { label: 'MCP工具调用', value: 'mcp_tool' },
  { label: 'HTTP请求', value: 'http_request' },
  { label: '条件判断', value: 'condition' },
  { label: '变量赋值', value: 'assign' },
  { label: '代码执行', value: 'code' },
];

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
    client.post<SkillExecution>(`/tenants/${tenantId}/skills/${skillId}/execute`, { inputs }),
  listExecutions: (tenantId: string, skillId?: string, limit = 20) =>
    client.get<{ executions: SkillExecution[] }>(`/tenants/${tenantId}/skill-executions`, { params: { skill_id: skillId, limit } }),
  getExecution: (executionId: string) =>
    client.get<SkillExecution>(`/skill-executions/${executionId}`),
};
