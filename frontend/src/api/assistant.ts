// AI助手 API
// SSE流式请求通过 fetch 直接调用，不走 axios
export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}
