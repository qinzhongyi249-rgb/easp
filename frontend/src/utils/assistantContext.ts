export type AssistantSurface = 'assistant_page' | 'floating_assistant';

export interface AssistantPageContext {
  surface: AssistantSurface;
  tenant_id: string;
  user_id?: string;
  route: {
    path: string;
    search: string;
    hash: string;
    title: string;
    label: string;
  };
  viewport: {
    width: number;
    height: number;
    is_mobile: boolean;
  };
  timestamp: string;
}

const routeLabels: Record<string, string> = {
  '/dashboard': '仪表盘',
  '/users': '用户管理',
  '/roles': '角色管理',
  '/tenants': '租户管理',
  '/connectors': '连接器',
  '/mcp-tools': 'MCP工具',
  '/skills': '技能管理',
  '/memory': '记忆管理',
  '/model-config': '模型配置',
  '/sso-config': 'SSO配置',
  '/assistant': 'AI助手',
  '/usage-analytics': '用量分析',
  '/audit-logs': '审计日志',
  '/api-keys': 'API Key',
};

export const getAssistantConversationKey = (surface: AssistantSurface, userId: string | undefined, tenantId: string) =>
  `easp_assistant_conversation_${surface}_${userId || 'anonymous'}_${tenantId}`;

export const loadAssistantConversationId = (surface: AssistantSurface, userId: string | undefined, tenantId: string): string => {
  try {
    return localStorage.getItem(getAssistantConversationKey(surface, userId, tenantId)) || '';
  } catch {
    return '';
  }
};

export const saveAssistantConversationId = (surface: AssistantSurface, userId: string | undefined, tenantId: string, conversationId: string) => {
  if (!conversationId) return;
  try {
    localStorage.setItem(getAssistantConversationKey(surface, userId, tenantId), conversationId);
  } catch { /* ignore */ }
};

export const clearAssistantConversationId = (surface: AssistantSurface, userId: string | undefined, tenantId: string) => {
  try {
    localStorage.removeItem(getAssistantConversationKey(surface, userId, tenantId));
  } catch { /* ignore */ }
};

export const buildAssistantPageContext = (surface: AssistantSurface, tenantId: string, userId?: string): AssistantPageContext => {
  const path = window.location.pathname;
  return {
    surface,
    tenant_id: tenantId,
    user_id: userId,
    route: {
      path,
      search: window.location.search,
      hash: window.location.hash,
      title: document.title,
      label: routeLabels[path] || path,
    },
    viewport: {
      width: window.innerWidth,
      height: window.innerHeight,
      is_mobile: window.innerWidth < 768,
    },
    timestamp: new Date().toISOString(),
  };
};
