import client from './client';

export interface MemoryPool {
  id: string;
  tenant_id: string;
  name: string;
  description: string;
  created_at: string;
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

export const memoryApi = {
  // Memory pools
  listPools: (tenantId: string) =>
    client.get<MemoryPool[]>(`/tenants/${tenantId}/memory-pools`),
  createPool: (tenantId: string, data: Partial<MemoryPool>) =>
    client.post<MemoryPool>(`/tenants/${tenantId}/memory-pools`, data),

  // Vector memories
  listMemories: (tenantId: string, poolId?: string, limit = 50) =>
    client.get<{ memories: VectorMemory[] }>(`/tenants/${tenantId}/vector-memories`, { params: { pool_id: poolId, limit } }),
  saveMemory: (tenantId: string, data: { pool_id?: string; content: string; type?: string; sensitivity?: string }) =>
    client.post<VectorMemory>(`/tenants/${tenantId}/vector-memories`, data),
  searchMemories: (tenantId: string, query: string, poolId?: string, limit = 10) =>
    client.get<{ memories: VectorMemory[] }>(`/tenants/${tenantId}/vector-memories/search`, { params: { q: query, pool_id: poolId, limit } }),
  deleteMemory: (tenantId: string, memoryId: string) =>
    client.delete(`/tenants/${tenantId}/vector-memories/${memoryId}`),
};
