export interface FeatureMenuPermission {
  label: string;
  value: string;
  path: string;
  desc: string;
}

// 统一维护：侧边栏功能菜单与角色「功能权限」选择项共用同一份配置，避免新增菜单后权限页漏同步。
// 不包含：dashboard（基础入口，所有登录用户可见）、tenants（系统管理员专属，不走租户角色 tools）。
export const FEATURE_MENU_PERMISSIONS: FeatureMenuPermission[] = [
  { label: '用户管理', value: 'users', path: '/users', desc: '管理租户下的用户' },
  { label: '角色管理', value: 'roles', path: '/roles', desc: '管理租户下的角色' },
  { label: '连接器', value: 'connectors', path: '/connectors', desc: '创建、编辑、删除连接器' },
  { label: 'MCP工具', value: 'mcp-tools', path: '/mcp-tools', desc: '管理MCP工具配置' },
  { label: '技能管理', value: 'skills', path: '/skills', desc: '创建和执行技能' },
  { label: '记忆管理', value: 'memory', path: '/memory', desc: '管理记忆池、长期记忆和向量记忆' },
  { label: '模型配置', value: 'model-config', path: '/model-config', desc: '配置AI模型和提供商' },
  { label: 'SSO配置', value: 'sso-config', path: '/sso-config', desc: '配置单点登录' },
  { label: 'AI 助手', value: 'assistant', path: '/assistant', desc: '使用平台内置AI助手' },
  { label: '用量分析', value: 'usage-analytics', path: '/usage-analytics', desc: '查看租户调用量、Token消耗和配额使用情况' },
  { label: '审计日志', value: 'audit-logs', path: '/audit-logs', desc: '查看操作审计日志' },
  { label: 'API Key', value: 'api-keys', path: '/api-keys', desc: '管理嵌入式聊天/API访问密钥' },
];
