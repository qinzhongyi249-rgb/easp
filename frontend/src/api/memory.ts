import client from './client';

export interface MemoryPool {
  id: string;
  tenant_id: string;
  name: string;
  description?: string;
  type: 'personal' | 'team' | 'system';
  purpose: 'conversation' | 'skill' | 'knowledge';
  priority: number; // 1-10
  max_tokens: number; // 0=不限
  auto_activate: boolean;
  trigger_rules?: string; // JSON
  owner_id?: string;
  enabled: boolean;
  memory_count: number;
  created_at: string;
  updated_at: string;
}

export interface VectorMemory {
  id: string;
  tenant_id: string;
  pool_id: string;
  content: string;
  type: string;
  sensitivity: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface UserMemory {
  id: string;
  tenant_id: string;
  user_id: string;
  pool_id?: string;
  type: string; // preference/fact/feedback
  content: string;
  entity_ids: string[];
  metadata: Record<string, unknown>;
  access_count: number;
  last_accessed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface Entity {
  id: string;
  tenant_id: string;
  pool_id?: string;
  name: string;
  type: string; // tenant/user/connector/tool/skill
  ref_id: string | null;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface SkillMemory {
  id: string;
  tenant_id: string;
  user_id: string | null;
  pool_id?: string;
  name: string;
  description: string | null;
  content: string;
  category: string | null;
  tags: string[];
  usage_count: number;
  created_at: string;
  updated_at: string;
}

export interface MemoryStats {
  total_user_memories: number;
  total_session_memories: number;
  total_entities: number;
  total_skill_memories: number;
  by_type: Record<string, number>;
}

export const memoryApi = {
  // Memory pools
  listPools: (tenantId: string) =>
    client.get<MemoryPool[]>(`/tenants/${tenantId}/memory-pools`),
  createPool: (tenantId: string, data: Partial<MemoryPool>) =>
    client.post<MemoryPool>(`/tenants/${tenantId}/memory-pools`, data),
  getPool: (poolId: string) =>
    client.get<MemoryPool>(`/memory-pools/${poolId}`),
  updatePool: (poolId: string, data: Partial<MemoryPool>) =>
    client.put<MemoryPool>(`/memory-pools/${poolId}`, data),
  deletePool: (poolId: string) =>
    client.delete(`/memory-pools/${poolId}`),

  // Vector memories
  listMemories: (tenantId: string, poolId?: string, limit = 50) =>
    client.get<{ memories: VectorMemory[] }>(`/tenants/${tenantId}/vector-memories`, { params: { pool_id: poolId, limit } }),
  saveMemory: (tenantId: string, data: { pool_id?: string; content: string; type?: string; sensitivity?: string }) =>
    client.post<VectorMemory>(`/tenants/${tenantId}/vector-memories`, data),
  searchMemories: (tenantId: string, query: string, poolId?: string, limit = 10) =>
    client.get<{ memories: VectorMemory[] }>(`/tenants/${tenantId}/vector-memories/search`, { params: { q: query, pool_id: poolId, limit } }),
  deleteMemory: (tenantId: string, memoryId: string) =>
    client.delete(`/tenants/${tenantId}/vector-memories/${memoryId}`),

  // User memories (all users in tenant)
  listAllUserMemories: (tenantId: string, limit = 50) =>
    client.get<{ memories: UserMemory[] }>(`/tenants/${tenantId}/user-memories`, { params: { limit } }),

  // Entities
  listEntities: (tenantId: string, limit = 50) =>
    client.get<{ entities: Entity[] }>(`/tenants/${tenantId}/entities`, { params: { limit } }),

  // Skill memories
  listSkillMemories: (tenantId: string, limit = 50) =>
    client.get<{ memories: SkillMemory[] }>(`/tenants/${tenantId}/skill-memories`, { params: { limit } }),

  // Memory stats
  getStats: (tenantId: string) =>
    client.get<MemoryStats>(`/tenants/${tenantId}/memory-stats`),
};
