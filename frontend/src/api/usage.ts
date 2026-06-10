import client from './client';

export interface ModelUsageStats {
  provider: string;
  model: string;
  today_tokens: number;
  month_tokens: number;
  today_calls: number;
  month_calls: number;
  month_input_tokens: number;
  month_output_tokens: number;
}

export interface UsageStats {
  // API 调用
  today_api_calls: number;
  month_api_calls: number;
  daily_quota: number;
  monthly_quota: number;
  rate_limit: number;
  // Token 消耗
  today_input_tokens: number;
  today_output_tokens: number;
  today_total_tokens: number;
  month_input_tokens: number;
  month_output_tokens: number;
  month_total_tokens: number;
  daily_token_quota: number;
  monthly_token_quota: number;
  // 按模型
  model_usage: ModelUsageStats[];
}

export const usageApi = {
  getStats: (tenantId: string) => client.get<UsageStats>(`/tenants/${tenantId}/usage`),
};
