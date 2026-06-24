import client from './client';

export interface TenantUser {
  id: string;
  user_uid?: string;
  account: string;
  tenant_id: string;
  email: string;
  phone?: string;
  display_name: string;
  avatar?: string;
  status: string;
  is_admin?: boolean;
  role_names?: string[];
  profile?: string | null;
  attributes?: string | null;
  created_at: string;
  last_login_at?: string;
  login_count: number;
  deleted_at?: string | null;
}

export interface UserIdentityBinding {
  id: string;
  tenant_id: string;
  user_id: string;
  provider: string;
  provider_user_id: string;
  union_id?: string;
  open_id?: string;
  external_system?: string;
  display_name?: string;
  avatar?: string;
  email?: string;
  phone?: string;
  metadata?: string | null;
  status: string;
  linked_at?: string;
  last_login_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface UserDetailResponse {
  user: TenantUser;
  identities: UserIdentityBinding[];
}

export interface ExternalUserBinding {
  id: string;
  tenant_id: string;
  user_id: string;
  external_system: string;
  external_user_id: string;
  display_name: string;
  email: string;
  phone: string;
  metadata?: string | null;
  status: string;
  last_login_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface TenantEmbedApp {
  id: string;
  tenant_id: string;
  app_id: string;
  name: string;
  external_system: string;
  allowed_origins?: string | null;
  allowed_scopes?: string | null;
  token_ttl_seconds: number;
  auto_create_user: boolean;
  default_role_ids?: string | null;
  status: string;
  last_used_at?: string | null;
  created_at: string;
  updated_at: string;
  app_secret?: string;
}

export interface CreateEmbedAppPayload {
  name: string;
  external_system: string;
  allowed_origins?: string[];
  allowed_scopes?: string[];
  token_ttl_seconds?: number;
  auto_create_user?: boolean;
  default_role_ids?: string[];
}

export interface EmbedAppGuide {
  tenant_id: string;
  app_id: string;
  app_name: string;
  external_system: string;
  allowed_origins: string[];
  allowed_scopes: string[];
  token_ttl_seconds: number;
  endpoints: { token_exchange: string; assistant_frame: string; sdk: string };
  examples: { iframe: string; sdk: string; signature_payload: Record<string, string> };
  warnings: string[];
}

export interface EmbedAppDiagnoseResult {
  can_issue_token: boolean;
  app_id: string;
  external_system: string;
  checks: Array<{ key: string; label: string; ok: boolean; code: string; suggestion: string }>;
}

export interface ImportExternalUsersPayload {
  external_system: string;
  default_password?: string;
  users: Array<{
    account?: string;
    external_user_id: string;
    password?: string;
    user_uid?: string;
    display_name?: string;
    email?: string;
    phone?: string;
    avatar?: string;
    department?: string;
    position?: string;
    tags?: string[];
    profile?: Record<string, unknown>;
    attributes?: Record<string, unknown>;
    identities?: Array<{
      provider: string;
      provider_user_id: string;
      union_id?: string;
      open_id?: string;
      display_name?: string;
      avatar?: string;
      email?: string;
      phone?: string;
      metadata?: Record<string, unknown>;
    }>;
    role_ids?: string[];
    metadata?: Record<string, unknown>;
  }>;
}

export const userApi = {
  listByTenant: (tenantId: string, params?: { keyword?: string; status?: string; limit?: number }) =>
    client.get<TenantUser[]>(`/tenants/${tenantId}/users`, { params }),
  get: (tenantId: string, userId: string) =>
    client.get<UserDetailResponse>(`/tenants/${tenantId}/users/${userId}`),
  create: (tenantId: string, data: { account: string; email?: string; password: string; display_name?: string; phone?: string }) =>
    client.post<TenantUser>(`/tenants/${tenantId}/users`, data),
  update: (tenantId: string, userId: string, data: Partial<TenantUser>) =>
    client.put<TenantUser>(`/tenants/${tenantId}/users/${userId}`, data),
  delete: (tenantId: string, userId: string) =>
    client.delete(`/tenants/${tenantId}/users/${userId}`),
  restore: (tenantId: string, userId: string) =>
    client.post(`/tenants/${tenantId}/users/${userId}/restore`),
  assignRole: (userId: string, roleId: string) =>
    client.post('/users/assign-role', { user_id: userId, role_id: roleId }),
  revokeRole: (userId: string, roleId: string) =>
    client.delete(`/users/${userId}/roles/${roleId}`),
  getRoles: (userId: string) =>
    client.get(`/users/${userId}/roles`),
  generateResetPassword: (tenantId: string, userId: string) =>
    client.post<{ password: string; saved: boolean }>(`/tenants/${tenantId}/users/${userId}/reset-password`),
  confirmResetPassword: (tenantId: string, userId: string, password: string) =>
    client.post<{ message: string; saved: boolean }>(`/tenants/${tenantId}/users/${userId}/reset-password`, { password }),
  listEmbedApps: (tenantId: string) =>
    client.get<TenantEmbedApp[]>(`/tenants/${tenantId}/embed-apps`),
  createEmbedApp: (tenantId: string, data: CreateEmbedAppPayload) =>
    client.post<{ app: TenantEmbedApp; app_secret: string }>(`/tenants/${tenantId}/embed-apps`, data),
  getEmbedAppGuide: (tenantId: string, appId: string) =>
    client.get<EmbedAppGuide>(`/tenants/${tenantId}/embed-apps/${appId}/guide`),
  diagnoseEmbedApp: (tenantId: string, appId: string, data: { origin?: string; external_user_id?: string }) =>
    client.post<EmbedAppDiagnoseResult>(`/tenants/${tenantId}/embed-apps/${appId}/diagnose`, data),
  listExternalUsers: (tenantId: string, params?: { external_system?: string; status?: string; keyword?: string; limit?: number }) =>
    client.get<ExternalUserBinding[]>(`/tenants/${tenantId}/external-users`, { params }),
  importExternalUsers: (tenantId: string, data: ImportExternalUsersPayload) =>
    client.post<{ items: Array<{ external_user_id: string; user_id?: string; user_uid?: string; account?: string; login_identifier?: string; password_configured?: boolean; password_updated?: boolean; status: string; error?: string }> }>(`/tenants/${tenantId}/external-users/import`, data),
  listUserIdentities: (tenantId: string, params?: { provider?: string; keyword?: string; limit?: number }) =>
    client.get<UserIdentityBinding[]>(`/tenants/${tenantId}/user-identities`, { params }),
};
