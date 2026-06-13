import client from './client';

export interface UsageStats {
  today_api_calls: number;
  month_api_calls: number;
  daily_quota: number;
  monthly_quota: number;
  rate_limit: number;
  today_input_tokens: number;
  today_output_tokens: number;
  today_cached_tokens: number;
  today_total_tokens: number;
  month_input_tokens: number;
  month_output_tokens: number;
  month_cached_tokens: number;
  month_total_tokens: number;
  daily_token_quota: number;
  monthly_token_quota: number;
  model_usage: ModelUsageStats[];
}

export interface ModelUsageStats {
  provider: string;
  model: string;
  today_tokens: number;
  month_tokens: number;
  today_calls: number;
  month_calls: number;
  month_input_tokens: number;
  month_output_tokens: number;
  month_cached_tokens: number;
}

export interface UsageAnalyticsSummary {
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  model_calls: number;
  tool_calls: number;
  mcp_tool_calls: number;
  skill_calls: number;
  builtin_tool_calls: number;
  avg_latency_ms: number;
}

export interface TokenTrendPoint {
  period: string;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  calls: number;
}

export interface UsageGroupStats {
  name: string;
  provider?: string;
  model?: string;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  calls: number;
  avg_latency_ms: number;
}

export interface ToolUsageStats {
  resource_type: string;
  resource_id: string;
  resource_name: string;
  source: string;
  calls: number;
  success_calls: number;
  failed_calls: number;
  avg_latency_ms: number;
}

export interface UsageDetail {
  kind: 'model' | 'tool' | string;
  id: number;
  user_id: string;
  source: string;
  source_name: string;
  provider: string;
  model: string;
  resource_type: string;
  resource_id: string;
  resource_name: string;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  latency_ms: number;
  status: string;
  request_id: string;
  error_message: string;
  created_at: string;
}

export interface UsageAnalyticsResponse {
  summary: UsageAnalyticsSummary;
  trend: TokenTrendPoint[];
  by_model: UsageGroupStats[];
  by_source: UsageGroupStats[];
  by_tool: ToolUsageStats[];
  details: UsageDetail[];
  page: number;
  page_size: number;
  total: number;
}

export interface UsageSummary {
  today_tokens: number;
  month_tokens: number;
  today_input_tokens: number;
  today_output_tokens: number;
  today_cached_tokens: number;
  today_model_calls: number;
  today_tool_calls: number;
  today_skill_calls: number;
}

export interface UsageAnalyticsParams {
  start_date?: string;
  end_date?: string;
  granularity?: 'day' | 'month' | 'year';
  source?: string;
  model_name?: string;
  resource_type?: string;
  page?: number;
  page_size?: number;
}

export const usageApi = {
  getStats: (tenantId: string) => client.get<UsageStats>(`/tenants/${tenantId}/usage`),
  analytics: (tenantId: string, params: UsageAnalyticsParams = {}) =>
    client.get<UsageAnalyticsResponse>(`/tenants/${tenantId}/usage/analytics`, { params }),
  summary: (tenantId: string) => client.get<UsageSummary>(`/tenants/${tenantId}/usage/summary`),
};
