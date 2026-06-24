import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Input, Button, Typography, Card, Space, Avatar, Tag, Tooltip } from 'antd';
import {
  SendOutlined, RobotOutlined, UserOutlined, ClearOutlined,
  LoadingOutlined, CheckCircleOutlined, ClockCircleOutlined,
  ThunderboltOutlined, ToolOutlined, BranchesOutlined,
  DownOutlined, ApiOutlined
} from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { MarkdownRenderer, MARKDOWN_CSS } from '../utils/markdown';
import {
  buildAssistantPageContext,
  clearAssistantConversationId,
  loadAssistantConversationId,
  saveAssistantConversationId,
} from '../utils/assistantContext';
import AccessManualModal from '../components/AccessManualModal';

const { Title, Text } = Typography;
const { TextArea } = Input;

interface LayoutContext { currentTenant: string; }

interface TraceStep {
  stage: string;
  message: string;
  elapsed_ms?: number;
  stage_ms?: number;
  tool_name?: string;
  tool_index?: number;
  tool_total?: number;
  timestamp: number;
}

interface ModelInfo {
  model: string;
  display_name: string;
  provider: string;
}

interface ToolResult {
  name: string;
  elapsed_ms: number;
}

interface DisplayMessage {
  role: 'user' | 'assistant' | 'status';
  content: string;
  isStreaming?: boolean;
  modelInfo?: ModelInfo;
  trace?: TraceStep[];
  toolResults?: ToolResult[];
  totalMs?: number;
}

const fmtMs = (ms?: number): string => {
  if (ms === undefined || ms === null || isNaN(ms) || ms < 0) return '';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
};

const stageConfig: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
  thinking: { icon: <LoadingOutlined />, color: '#1677ff', label: '模型思考' },
  plan: { icon: <BranchesOutlined />, color: '#722ed1', label: '工具规划' },
  tool_calling: { icon: <ToolOutlined />, color: '#fa8c16', label: '执行工具' },
  generating: { icon: <ThunderboltOutlined />, color: '#52c41a', label: '生成回答' },
};

const TYPEWRITER_FRAME_MS = 24;
const TYPEWRITER_CHARS_PER_FRAME = 3;

const appendAssistantStreamingMessage = (
  updater: React.Dispatch<React.SetStateAction<DisplayMessage[]>>,
  content: string,
  modelInfo?: ModelInfo | null,
  traceSteps: TraceStep[] = [],
  toolResults: ToolResult[] = [],
) => {
  updater(prev => {
    const filtered = prev.filter(m => m.role !== 'status');
    const last = filtered[filtered.length - 1];
    const nextMsg: DisplayMessage = {
      role: 'assistant',
      content,
      isStreaming: true,
      modelInfo: modelInfo || undefined,
      trace: traceSteps.length > 0 ? traceSteps : undefined,
      toolResults: toolResults.length > 0 ? toolResults : undefined,
    };
    if (last?.role === 'assistant' && last.isStreaming) {
      const next = [...filtered];
      next[next.length - 1] = { ...last, ...nextMsg };
      return next;
    }
    return [...filtered, nextMsg];
  });
};

const TraceTimeline: React.FC<{
  trace: TraceStep[];
  toolResults: ToolResult[];
  totalMs?: number;
  isStreaming?: boolean;
}> = ({ trace, toolResults, totalMs, isStreaming }) => {
  const [expanded, setExpanded] = useState(false);
  const [, setTick] = useState(0);

  useEffect(() => {
    if (expanded || !isStreaming || totalMs !== undefined) return;
    const timer = setInterval(() => setTick(t => t + 1), 200);
    return () => clearInterval(timer);
  }, [expanded, isStreaming, totalMs]);

  const uniqueSteps: TraceStep[] = [];
  const seen = new Set<string>();
  for (let i = trace.length - 1; i >= 0; i--) {
    const key = trace[i].stage + (trace[i].tool_name || '') + (trace[i].tool_index || '');
    if (!seen.has(key)) {
      seen.add(key);
      uniqueSteps.unshift(trace[i]);
    }
  }

  const firstTimestamp = uniqueSteps.length > 0 && uniqueSteps[0].timestamp ? uniqueSteps[0].timestamp : Date.now();
  const liveElapsed = totalMs ?? Math.max(0, Date.now() - firstTimestamp);

  const lastStep = uniqueSteps[uniqueSteps.length - 1];
  const summaryText = lastStep?.stage === 'generating' ? '生成中...' :
    lastStep?.stage === 'tool_calling' ? `执行工具 ${lastStep.tool_index}/${lastStep.tool_total}` :
    lastStep?.message || '处理中...';

  return (
    <div style={{ marginBottom: 8 }}>
      <div 
        style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer', padding: '4px 0' }}
        onClick={() => setExpanded(!expanded)}
      >
        {expanded ? <DownOutlined style={{ fontSize: 10 }} /> : <DownOutlined style={{ fontSize: 10, transform: 'rotate(-90deg)' }} />}
        <ClockCircleOutlined style={{ color: '#8c8c8c', fontSize: 12 }} />
        <Text type="secondary" style={{ fontSize: 12 }}>
          {summaryText} · {fmtMs(liveElapsed)}
        </Text>
      </div>
      {expanded && (
        <div style={{ paddingLeft: 16, borderLeft: '2px solid #f0f0f0', marginLeft: 4 }}>
          {uniqueSteps.map((step, i) => {
            const config = stageConfig[step.stage] || stageConfig.thinking;
            return (
              <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0' }}>
                <span style={{ color: config.color }}>{config.icon}</span>
                <Text style={{ fontSize: 12, flex: 1 }}>
                  {step.tool_name ? `${config.label}: ${step.tool_name}` : config.label}
                </Text>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {fmtMs(step.stage_ms)}
                </Text>
              </div>
            );
          })}
          {toolResults.length > 0 && (
            <div style={{ marginTop: 4, padding: '4px 8px', background: '#f6f6f6', borderRadius: 4 }}>
              <Text type="secondary" style={{ fontSize: 11 }}>
                工具调用: {toolResults.map(t => `${t.name}(${fmtMs(t.elapsed_ms)})`).join(', ')}
              </Text>
            </div>
          )}
          {totalMs !== undefined && (
            <div style={{ marginTop: 4 }}>
              <Text type="secondary" style={{ fontSize: 11 }}>总耗时: {fmtMs(totalMs)}</Text>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

const Assistant: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const { user } = useAuth();
  const [input, setInput] = useState('');
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [sending, setSending] = useState(false);
  const [currentModel, setCurrentModel] = useState<ModelInfo | null>(null);
  const [manualOpen, setManualOpen] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const isMobile = window.innerWidth < 768;

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const onSend = useCallback(async () => {
    if (!input.trim() || sending) return;
    const userMsg: DisplayMessage = { role: 'user', content: input.trim() };
    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setSending(true);

    const statusMsg: DisplayMessage = { role: 'status', content: '思考中...', trace: [], toolResults: [] };
    setMessages(prev => [...prev, statusMsg]);

    try {
      abortControllerRef.current = new AbortController();
      const conversationId = loadAssistantConversationId('assistant_page', user?.id, currentTenant);
      const res = await fetch(`/api/v1/tenants/${currentTenant}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'Authorization': `Bearer ${localStorage.getItem('access_token')}` },
        body: JSON.stringify({
          conversation_id: conversationId || undefined,
          page_context: buildAssistantPageContext('assistant_page', currentTenant, user?.id),
          messages: [{ role: 'user', content: input.trim() }],
        }),
        signal: abortControllerRef.current.signal,
      });

      if (!res.ok) throw new Error(`HTTP ${res.status}`);

      const reader = res.body?.getReader();
      if (!reader) throw new Error('No reader');
      const decoder = new TextDecoder();
      let buffer = '';
      let currentEvent = '';
      let traceSteps: TraceStep[] = [];
      let toolResults: ToolResult[] = [];
      let modelInfo: ModelInfo | null = null;
      let totalMs: number | undefined;
      const typewriterQueueRef = { current: '' };
      let displayedContent = '';
      let typewriterTimer: number | null = null;

      const stopTypewriter = () => {
        if (typewriterTimer !== null) {
          window.clearInterval(typewriterTimer);
          typewriterTimer = null;
        }
      };
      const renderAssistantContent = (content: string) => {
        appendAssistantStreamingMessage(setMessages, content, modelInfo, traceSteps, toolResults);
      };
      const drainTypewriter = (force = false) => {
        if (!typewriterQueueRef.current) return;
        const chars = Array.from(typewriterQueueRef.current);
        const take = force ? chars.length : Math.min(TYPEWRITER_CHARS_PER_FRAME, chars.length);
        displayedContent += chars.slice(0, take).join('');
        typewriterQueueRef.current = chars.slice(take).join('');
        renderAssistantContent(displayedContent);
        if (!typewriterQueueRef.current) stopTypewriter();
      };
      const enqueueDelta = (piece: string) => {
        if (!piece) return;
        typewriterQueueRef.current += piece;
        if (typewriterTimer === null) {
          typewriterTimer = window.setInterval(() => drainTypewriter(false), TYPEWRITER_FRAME_MS);
        }
      };
      const flushTypewriter = () => {
        drainTypewriter(true);
        stopTypewriter();
      };

      const updateStatus = (content: string, trace?: TraceStep[]) => {
        setMessages(prev => {
          const updated = [...prev];
          const last = updated[updated.length - 1];
          const nextStatus: DisplayMessage = {
            role: 'status',
            content,
            trace: trace || traceSteps,
            toolResults,
          };
          if (last?.role === 'status') {
            updated[updated.length - 1] = { ...last, ...nextStatus };
            return updated;
          }
          return [...updated, nextStatus];
        });
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const rawLine of lines) {
          const line = rawLine.trimEnd();
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim();
            continue;
          }
          if (!line.startsWith('data: ')) continue;
          try {
            const data = JSON.parse(line.slice(6));
            const eventType = currentEvent || data.type || 'message';
            currentEvent = '';
            switch (eventType) {
              case 'conversation':
                if (data.conversation_id) {
                  saveAssistantConversationId('assistant_page', user?.id, currentTenant, data.conversation_id);
                }
                break;
              case 'model_info':
              case 'model':
                modelInfo = { model: data.model, display_name: data.display_name, provider: data.provider };
                setCurrentModel(modelInfo);
                break;
              case 'status':
              case 'heartbeat':
              case 'trace': {
                traceSteps = [...traceSteps, { ...data, timestamp: data.timestamp || Date.now() }];
                updateStatus(data.message || '处理中...', traceSteps);
                break;
              }
              case 'tool':
              case 'tool_result':
                toolResults = [...toolResults, { name: data.name, elapsed_ms: data.elapsed_ms }];
                updateStatus(`✓ ${data.name} 完成`, traceSteps);
                break;
              case 'delta':
                enqueueDelta(data.content || '');
                break;
              case 'done':
                flushTypewriter();
                totalMs = data?.total_ms;
                setMessages(prev => {
                  const filtered = prev.filter(m => m.role !== 'status');
                  if (!displayedContent) return filtered;
                  const last = filtered[filtered.length - 1];
                  const nextMsg: DisplayMessage = {
                    role: 'assistant',
                    content: displayedContent,
                    isStreaming: false,
                    modelInfo: modelInfo || undefined,
                    trace: traceSteps.length > 0 ? traceSteps : undefined,
                    toolResults: toolResults.length > 0 ? toolResults : undefined,
                    totalMs,
                  };
                  if (last?.role === 'assistant') {
                    const next = [...filtered];
                    next[next.length - 1] = { ...last, ...nextMsg };
                    return next;
                  }
                  return [...filtered, nextMsg];
                });
                break;
              case 'error':
                flushTypewriter();
                setMessages(prev => {
                  const filtered = prev.filter(m => m.role !== 'status');
                  return [...filtered, { role: 'assistant', content: `❌ ${data.message}` }];
                });
                break;
            }
          } catch {
            // 忽略解析错误
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
  }, [input, sending, messages, currentTenant, user?.id]);

  const onClear = () => {
    abortControllerRef.current?.abort();
    clearAssistantConversationId('assistant_page', user?.id, currentTenant);
    setMessages([]);
    setSending(false);
    setCurrentModel(null);
  };

  const quickCommands = [
    { label: '👥 用户管理', cmd: '请帮我查看当前租户下的所有用户和角色' },
    { label: '🔐 角色分配', cmd: '请帮我给 admin@easp.com 分配管理员角色' },
    { label: '📊 租户概览', cmd: '请帮我全面了解当前租户的状态' },
    { label: '🔌 连接器诊断', cmd: '请帮我检查连接器和MCP工具的配置状态' },
    { label: '🔧 查看MCP工具', cmd: '请帮我查看当前租户下的MCP工具' },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: isMobile ? 'calc(100vh - 80px)' : 'calc(100vh - 120px)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: isMobile ? 'flex-start' : 'center', marginBottom: 16, flexDirection: isMobile ? 'column' : 'row', gap: isMobile ? 8 : 0 }}>
        <Title level={isMobile ? 4 : 3} style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
          <RobotOutlined /> AI 助手
          {currentModel && (
            <Tooltip title={`模型: ${currentModel.model} | 供应商: ${currentModel.provider}`}>
              <Tag
                icon={<ThunderboltOutlined />}
                color="blue"
                style={{ fontSize: 12, cursor: 'pointer' }}
              >
                {currentModel.display_name || currentModel.model}
              </Tag>
            </Tooltip>
          )}
          {sending && <Tag icon={<LoadingOutlined />} color="processing" style={{ marginLeft: 4 }}>思考中</Tag>}
        </Title>
        <Space wrap>
          <Button icon={<ApiOutlined />} onClick={() => setManualOpen(true)} size={isMobile ? 'small' : 'middle'}>
            AI助手接入手册
          </Button>
          <Button icon={<ClearOutlined />} onClick={onClear} disabled={messages.length === 0} size={isMobile ? 'small' : 'middle'}>
            清空对话
          </Button>
        </Space>
      </div>

      <Card 
        style={{ flex: 1, overflow: 'auto', marginBottom: 16, background: '#fafafa' }}
        bodyStyle={{ padding: isMobile ? 8 : 16 }}
      >
        {messages.length === 0 ? (
          <div style={{ textAlign: 'center', padding: isMobile ? '20px 0' : '40px 0' }}>
            <RobotOutlined style={{ fontSize: isMobile ? 32 : 48, color: '#d9d9d9', marginBottom: 16 }} />
            <div><Text type="secondary">你好！我是 EASP 智能助手，可以帮你管理平台资源。</Text></div>
            <div style={{ marginTop: 8 }}><Text type="secondary">试试下面的快捷命令，或者直接输入你的需求：</Text></div>
            <div style={{ marginTop: 24, display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 8 }}>
              {quickCommands.map((cmd, i) => (
                <Button key={i} size="small" onClick={() => setInput(cmd.cmd)}>
                  {cmd.label}
                </Button>
              ))}
            </div>
          </div>
        ) : (
          <div>
            {messages.map((msg, index) => (
              <div 
                key={index} 
                style={{ 
                  display: 'flex', 
                  justifyContent: msg.role === 'user' ? 'flex-end' : 'flex-start',
                  marginBottom: 16,
                }}
              >
                {msg.role === 'assistant' && (
                  <Avatar icon={<RobotOutlined />} style={{ backgroundColor: '#1677ff', marginRight: 8, flexShrink: 0 }} size={isMobile ? 'small' : 'default'} />
                )}
                {msg.role === 'status' && (
                  <Avatar icon={<LoadingOutlined />} style={{ backgroundColor: '#faad14', marginRight: 8, flexShrink: 0 }} size={isMobile ? 'small' : 'default'} />
                )}
                <div style={{ 
                  maxWidth: isMobile ? '85%' : '75%',
                  padding: isMobile ? '8px 12px' : '12px 16px',
                  borderRadius: 12,
                  background: msg.role === 'user' ? '#1677ff' : msg.role === 'status' ? '#fffbe6' : '#fff',
                  color: msg.role === 'user' ? '#fff' : '#333',
                  boxShadow: '0 1px 2px rgba(0,0,0,0.1)',
                  border: msg.role === 'status' ? '1px solid #ffe58f' : 'none',
                  fontSize: isMobile ? 13 : 14,
                }}>
                  {msg.role === 'status' ? (
                    <div>
                      {msg.trace && msg.trace.length > 0 && (
                        <TraceTimeline
                          trace={msg.trace}
                          toolResults={msg.toolResults || []}
                          isStreaming={true}
                        />
                      )}
                      <Space>
                        <LoadingOutlined style={{ color: '#faad14' }} />
                        <Text style={{ color: '#8c6e00' }}>{msg.content}</Text>
                      </Space>
                    </div>
                  ) : msg.role === 'assistant' ? (
                    <>
                      {msg.trace && msg.trace.length > 0 && (
                        <TraceTimeline
                          trace={msg.trace}
                          toolResults={msg.toolResults || []}
                          totalMs={msg.totalMs}
                          isStreaming={msg.isStreaming}
                        />
                      )}
                      <MarkdownRenderer content={msg.content} />
                      {msg.isStreaming && <span className="typing-cursor">▊</span>}
                    </>
                  ) : (
                    <div style={{ whiteSpace: 'pre-wrap' }}>{msg.content}</div>
                  )}
                </div>
                {msg.role === 'user' && (
                  <Avatar icon={<UserOutlined />} style={{ backgroundColor: '#87d068', marginLeft: 8, flexShrink: 0 }} size={isMobile ? 'small' : 'default'} />
                )}
                {msg.role === 'assistant' && !msg.isStreaming && msg.content && (
                  <CheckCircleOutlined style={{ color: '#52c41a', marginLeft: 8, alignSelf: 'center' }} />
                )}
              </div>
            ))}
            <div ref={messagesEndRef} />
          </div>
        )}
      </Card>

      <div style={{ display: 'flex', gap: isMobile ? 4 : 8 }}>
        <TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="输入你的需求..."
          autoSize={{ minRows: 1, maxRows: isMobile ? 2 : 4 }}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              onSend();
            }
          }}
          disabled={sending}
          style={{ flex: 1 }}
          size={isMobile ? 'small' : 'middle'}
        />
        <Button 
          type="primary" 
          icon={<SendOutlined />} 
          onClick={onSend} 
          loading={sending}
          style={{ height: 'auto' }}
          size={isMobile ? 'small' : 'middle'}
        >
          {isMobile ? '' : '发送'}
        </Button>
      </div>

      <AccessManualModal type="assistant" open={manualOpen} tenantId={currentTenant} onClose={() => setManualOpen(false)} />

      <style>{`
        .typing-cursor {
          animation: blink 0.8s infinite;
          color: #1677ff;
          font-weight: bold;
        }
        @keyframes blink {
          0%, 100% { opacity: 1; }
          50% { opacity: 0; }
        }
        ${MARKDOWN_CSS}
      `}</style>
    </div>
  );
};

export default Assistant;
