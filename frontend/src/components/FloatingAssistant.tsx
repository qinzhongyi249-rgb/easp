import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Input, Button, Typography, Space, Avatar, Tag, Tooltip, Drawer, Badge } from 'antd';
import {
  SendOutlined, RobotOutlined, UserOutlined, ClearOutlined,
  LoadingOutlined, CheckCircleOutlined, CloseOutlined,
  CustomerServiceOutlined,
} from '@ant-design/icons';
import { MarkdownRenderer, MARKDOWN_CSS } from '../utils/markdown';

const { Text } = Typography;
const { TextArea } = Input;

interface ModelInfo {
  model: string;
  display_name: string;
  provider: string;
}

interface DisplayMessage {
  role: 'user' | 'assistant' | 'status';
  content: string;
  isStreaming?: boolean;
  modelInfo?: ModelInfo;
  totalMs?: number;
}

interface FloatingAssistantProps {
  tenantId: string;
}

const FloatingAssistant: React.FC<FloatingAssistantProps> = ({ tenantId }) => {
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [currentModel, setCurrentModel] = useState<ModelInfo | null>(null);
  const [unread, setUnread] = useState(0);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (open) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
      setUnread(0);
    }
  }, [messages, open]);

  // SSE 流式请求
  const onSend = useCallback(async () => {
    const text = input.trim();
    if (!text || sending) return;

    const userMsg: DisplayMessage = { role: 'user', content: text };
    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setSending(true);

    const controller = new AbortController();
    abortControllerRef.current = controller;

    try {
      const history = messages
        .filter(m => m.role === 'user' || (m.role === 'assistant' && !m.isStreaming))
        .map(m => ({ role: m.role === 'status' ? 'assistant' : m.role, content: m.content }));
      history.push({ role: 'user', content: text });

      const token = localStorage.getItem('access_token');
      const response = await fetch(`/api/v1/tenants/${tenantId}/chat`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ messages: history }),
        signal: controller.signal,
      });

      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body?.getReader();
      if (!reader) throw new Error('No reader');

      const decoder = new TextDecoder();
      let buffer = '';
      let currentContent = '';
      let modelInfo: ModelInfo | null = null;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            const dataStr = line.slice(6);
            let eventType = 'unknown';
            const lineIndex = lines.indexOf(line);
            for (let i = lineIndex - 1; i >= 0; i--) {
              if (lines[i].startsWith('event: ')) {
                eventType = lines[i].slice(7).trim();
                break;
              }
            }

            try {
              const data = JSON.parse(dataStr);

              switch (eventType) {
                case 'model_info':
                  modelInfo = data;
                  setCurrentModel(data);
                  break;

                case 'status': {
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    return [...filtered, {
                      role: 'status',
                      content: data.message || '处理中...',
                    }];
                  });
                  break;
                }

                case 'tool':
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    return [...filtered, {
                      role: 'status',
                      content: `✓ ${data.name} 完成`,
                    }];
                  });
                  break;

                case 'delta':
                  currentContent += data.content;
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    const lastMsg = filtered[filtered.length - 1];
                    if (lastMsg && lastMsg.role === 'assistant' && lastMsg.isStreaming) {
                      const newMsgs = [...filtered];
                      newMsgs[newMsgs.length - 1] = {
                        role: 'assistant', content: currentContent,
                        isStreaming: true, modelInfo: modelInfo || undefined,
                      };
                      return newMsgs;
                    }
                    return [...filtered, {
                      role: 'assistant', content: currentContent,
                      isStreaming: true, modelInfo: modelInfo || undefined,
                    }];
                  });
                  break;

                case 'done': {
                  const totalMs = data.total_ms;
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    const lastMsg = filtered[filtered.length - 1];
                    if (lastMsg && lastMsg.role === 'assistant' && lastMsg.isStreaming) {
                      const newMsgs = [...filtered];
                      newMsgs[newMsgs.length - 1] = {
                        ...lastMsg, isStreaming: false, totalMs,
                        modelInfo: modelInfo || undefined,
                      };
                      return newMsgs;
                    }
                    return filtered;
                  });
                  if (!open) setUnread(u => u + 1);
                  currentContent = '';
                  break;
                }

                case 'error':
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    return [...filtered, { role: 'assistant', content: `❌ ${data.message}` }];
                  });
                  break;
              }
            } catch { /* ignore parse errors */ }
          }
        }
      }
    } catch (err: unknown) {
      if ((err as Error).name === 'AbortError') return;
      const e = err as { message?: string };
      setMessages(prev => {
        const filtered = prev.filter(m => m.role !== 'status');
        return [...filtered, { role: 'assistant', content: `❌ 请求失败: ${e.message || '未知错误'}` }];
      });
    } finally {
      setSending(false);
      abortControllerRef.current = null;
    }
  }, [input, sending, messages, tenantId, open]);

  const onClear = () => {
    abortControllerRef.current?.abort();
    setMessages([]);
    setSending(false);
    setCurrentModel(null);
  };

  return (
    <>
      {/* 浮动按钮 */}
      <Badge count={unread} size="small" offset={[-4, 4]}>
        <div
          onClick={() => setOpen(true)}
          style={{
            position: 'fixed', right: 24, bottom: 24, zIndex: 1000,
            width: 56, height: 56, borderRadius: '50%',
            background: 'linear-gradient(135deg, #1677ff, #4096ff)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: 'pointer', boxShadow: '0 4px 16px rgba(22,119,255,0.4)',
            transition: 'all 0.3s',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.transform = 'scale(1.1)';
            e.currentTarget.style.boxShadow = '0 6px 24px rgba(22,119,255,0.5)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.transform = 'scale(1)';
            e.currentTarget.style.boxShadow = '0 4px 16px rgba(22,119,255,0.4)';
          }}
        >
          <CustomerServiceOutlined style={{ fontSize: 26, color: '#fff' }} />
        </div>
      </Badge>

      {/* 聊天抽屉 */}
      <Drawer
        title={
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <RobotOutlined style={{ color: '#1677ff' }} />
            <span>AI 助手</span>
            {currentModel && (
              <Tooltip title={`${currentModel.model} | ${currentModel.provider}`}>
                <Tag color="blue" style={{ fontSize: 11, cursor: 'pointer', margin: 0 }}>
                  {currentModel.display_name || currentModel.model}
                </Tag>
              </Tooltip>
            )}
            {sending && <Tag icon={<LoadingOutlined />} color="processing" style={{ margin: 0, fontSize: 11 }}>思考中</Tag>}
          </div>
        }
        placement="right"
        width={420}
        open={open}
        onClose={() => setOpen(false)}
        closeIcon={<CloseOutlined />}
        styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column', height: 'calc(100vh - 55px)' } }}
        extra={
          <Button icon={<ClearOutlined />} size="small" onClick={onClear} disabled={messages.length === 0}>
            清空
          </Button>
        }
      >
        {/* 消息区域 */}
        <div style={{ flex: 1, overflow: 'auto', padding: 16, background: '#f8f9fa' }}>
          {messages.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '60px 20px' }}>
              <RobotOutlined style={{ fontSize: 48, color: '#d9d9d9', marginBottom: 16 }} />
              <div><Text type="secondary">你好！我是 EASP 智能助手</Text></div>
              <div style={{ marginTop: 8 }}><Text type="secondary" style={{ fontSize: 12 }}>输入你的需求开始对话</Text></div>
            </div>
          ) : (
            <div>
              {messages.map((msg, index) => (
                <div
                  key={index}
                  style={{
                    display: 'flex',
                    justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
                    marginBottom: 12,
                  }}
                >
                  {msg.role !== 'user' && (
                    <Avatar
                      icon={msg.role === 'status' ? <LoadingOutlined /> : <RobotOutlined />}
                      size={28}
                      style={{
                        backgroundColor: msg.role === 'status' ? '#faad14' : '#1677ff',
                        marginRight: 8, flexShrink: 0,
                      }}
                    />
                  )}
                  <div style={{
                    maxWidth: '80%', padding: '8px 12px', borderRadius: 12,
                    background: msg.role === 'user' ? '#1677ff' : msg.role === 'status' ? '#fffbe6' : '#fff',
                    color: msg.role === 'user' ? '#fff' : '#333',
                    boxShadow: '0 1px 2px rgba(0,0,0,0.06)',
                    border: msg.role === 'status' ? '1px solid #ffe58f' : 'none',
                    fontSize: 13,
                  }}>
                    {msg.role === 'status' ? (
                      <Space size={6}>
                        <LoadingOutlined style={{ color: '#faad14', fontSize: 12 }} />
                        <Text style={{ color: '#8c6e00', fontSize: 12 }}>{msg.content}</Text>
                      </Space>
                    ) : msg.role === 'assistant' ? (
                      <>
                        <MarkdownRenderer content={msg.content} compact />
                        {msg.isStreaming && <span style={{ color: '#1677ff', fontWeight: 'bold', animation: 'blink 0.8s infinite' }}>▊</span>}
                        {!msg.isStreaming && msg.totalMs !== undefined && (
                          <div style={{ marginTop: 4, fontSize: 11, color: '#999', fontFamily: 'monospace' }}>
                            {msg.totalMs < 1000 ? `${msg.totalMs}ms` : `${(msg.totalMs / 1000).toFixed(1)}s`}
                          </div>
                        )}
                      </>
                    ) : (
                      <div style={{ whiteSpace: 'pre-wrap' }}>{msg.content}</div>
                    )}
                  </div>
                  {msg.role === 'user' && (
                    <Avatar icon={<UserOutlined />} size={28} style={{ backgroundColor: '#87d068', marginLeft: 8, flexShrink: 0 }} />
                  )}
                  {msg.role === 'assistant' && !msg.isStreaming && msg.content && (
                    <CheckCircleOutlined style={{ color: '#52c41a', marginLeft: 6, alignSelf: 'center', fontSize: 12 }} />
                  )}
                </div>
              ))}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>

        {/* 输入区域 */}
        <div style={{ padding: '12px 16px', borderTop: '1px solid #f0f0f0', background: '#fff' }}>
          <div style={{ display: 'flex', gap: 8 }}>
            <TextArea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="输入你的需求..."
              autoSize={{ minRows: 1, maxRows: 3 }}
              onPressEnter={(e) => {
                if (!e.shiftKey) { e.preventDefault(); onSend(); }
              }}
              disabled={sending}
              style={{ flex: 1 }}
            />
            <Button
              type="primary"
              icon={<SendOutlined />}
              onClick={onSend}
              loading={sending}
              style={{ height: 'auto' }}
            />
          </div>
        </div>
      </Drawer>

      <style>{`
        @keyframes blink {
          0%, 100% { opacity: 1; }
          50% { opacity: 0; }
        }
        ${MARKDOWN_CSS}
      `}</style>
    </>
  );
};

export default FloatingAssistant;
