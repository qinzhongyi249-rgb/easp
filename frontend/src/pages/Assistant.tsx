import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Input, Button, Typography, Card, Space, Avatar, Tag, Tooltip } from 'antd';
import {
  SendOutlined, RobotOutlined, UserOutlined, ClearOutlined,
  LoadingOutlined, CheckCircleOutlined, ClockCircleOutlined,
  ThunderboltOutlined, ToolOutlined, BranchesOutlined,
  DownOutlined
} from '@ant-design/icons';
import { useOutletContext } from 'react-router-dom';
import { MarkdownRenderer, MARKDOWN_CSS } from '../utils/markdown';

const { Title, Text } = Typography;
const { TextArea } = Input;

interface LayoutContext { currentTenant: string; }

interface TraceStep {
  stage: string;       // thinking | plan | tool_calling | generating
  message: string;
  elapsed_ms?: number; // 从请求开始的累计时间（用于时间线位置）
  stage_ms?: number;   // 本阶段实际耗时
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

// 格式化耗时
const fmtMs = (ms?: number): string => {
  if (ms === undefined || ms === null || isNaN(ms) || ms < 0) return '';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
};

// 阶段配置
const stageConfig: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
  thinking: { icon: <LoadingOutlined />, color: '#1677ff', label: '模型思考' },
  plan: { icon: <BranchesOutlined />, color: '#722ed1', label: '工具规划' },
  tool_calling: { icon: <ToolOutlined />, color: '#fa8c16', label: '执行工具' },
  generating: { icon: <ThunderboltOutlined />, color: '#52c41a', label: '生成回答' },
};

// 气泡链路组件
const TraceTimeline: React.FC<{
  trace: TraceStep[];
  toolResults: ToolResult[];
  totalMs?: number;
  isStreaming?: boolean;
}> = ({ trace, toolResults, totalMs, isStreaming }) => {
  const [expanded, setExpanded] = useState(false);
  const [, setTick] = useState(0);

  // 摘要行实时滚动计时 - 仅在未展开 + 流式阶段生效
  useEffect(() => {
    if (expanded || !isStreaming || totalMs !== undefined) return;
    const timer = setInterval(() => setTick(t => t + 1), 200);
    return () => clearInterval(timer);
  }, [expanded, isStreaming, totalMs]);

  // 去重：只保留每个阶段的最新状态
  const uniqueSteps: TraceStep[] = [];
  const seen = new Set<string>();
  for (let i = trace.length - 1; i >= 0; i--) {
    const key = trace[i].stage + (trace[i].tool_name || '') + (trace[i].tool_index || '');
    if (!seen.has(key)) {
      seen.add(key);
      uniqueSteps.unshift(trace[i]);
    }
  }

  // 计算实时总耗时（摘要行用）
  const firstTimestamp = uniqueSteps.length > 0 && uniqueSteps[0].timestamp ? uniqueSteps[0].timestamp : Date.now();
  const liveElapsed = totalMs ?? Math.max(0, Date.now() - firstTimestamp);

  // 汇总文本
  const lastStep = uniqueSteps[uniqueSteps.length - 1];
  const summaryText = lastStep?.stage === 'generating' ? '生成中...' :
    lastStep?.stage === 'tool_calling' ? `执行工具 ${lastStep.tool_index}/${lastStep.tool_total}` :
    lastStep?.message || '处理中...';

  return (
    <div style={{ marginBottom: 8 }}>
      {/* 紧凑摘要行 - 始终显示实时滚动的总耗时 */}
      <div
        onClick={() => setExpanded(!expanded)}
        style={{
          display: 'inline-flex', alignItems: 'center', gap: 6,
          padding: '3px 10px', borderRadius: 12,
          background: '#f6f8fa', border: '1px solid #e8e8e8',
          cursor: 'pointer', fontSize: 12, color: '#666',
          userSelect: 'none', maxWidth: '100%', flexWrap: 'wrap',
        }}
      >
        <ClockCircleOutlined style={{ color: '#999' }} />
        <span>{summaryText}</span>
        <Tag style={{
          margin: 0, fontSize: 11, lineHeight: '18px',
          fontFamily: 'monospace', minWidth: 60, textAlign: 'center',
          color: totalMs !== undefined ? '#52c41a' : '#1677ff',
          borderColor: totalMs !== undefined ? '#52c41a' : '#1677ff',
        }}>
          {totalMs !== undefined ? fmtMs(totalMs) : `⏱ ${fmtMs(liveElapsed)}`}
        </Tag>
        <DownOutlined style={{
          fontSize: 10, color: '#999',
          transform: expanded ? 'rotate(180deg)' : 'rotate(0)',
          transition: 'transform 0.2s',
        }} />
      </div>

      {/* 展开的详细链路 - 各阶段只显示完成后的耗时 */}
      {expanded && (
        <div style={{
          marginTop: 6, padding: '8px 12px', borderRadius: 8,
          background: '#fafbfc', border: '1px solid #e8e8e8',
          fontSize: 12,
        }}>
          {uniqueSteps.map((step, i) => {
            const cfg = stageConfig[step.stage] || stageConfig.thinking;
            const isLast = i === uniqueSteps.length - 1;
            const isRunning = isLast && isStreaming && totalMs === undefined;
            const isDone = step.stage_ms !== undefined && step.stage_ms !== null;

            return (
              <div key={i} style={{
                display: 'flex', alignItems: 'flex-start', gap: 8,
                paddingBottom: i < uniqueSteps.length - 1 ? 6 : 0,
                position: 'relative',
              }}>
                {/* 时间线竖线 */}
                {!isLast && (
                  <div style={{
                    position: 'absolute', left: 9, top: 20, bottom: -6,
                    width: 1, background: isDone ? '#52c41a30' : '#e0e0e0',
                  }} />
                )}
                {/* 阶段图标 */}
                <div style={{
                  width: 18, height: 18, borderRadius: '50%',
                  background: isDone ? '#52c41a15' : cfg.color + '20',
                  display: 'flex', alignItems: 'center', justifyContent: 'center',
                  flexShrink: 0, zIndex: 1,
                  color: isDone ? '#52c41a' : cfg.color, fontSize: 10,
                }}>
                  {isRunning ? <LoadingOutlined spin /> : isDone ? <CheckCircleOutlined /> : cfg.icon}
                </div>
                {/* 内容 */}
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
                    <Text strong style={{ fontSize: 12 }}>
                      {step.tool_name ? `${cfg.label}: ${step.tool_name}` : cfg.label}
                    </Text>
                    {/* 完成的阶段显示耗时 */}
                    {isDone && (
                      <Tag style={{
                        margin: 0, fontSize: 10, lineHeight: '16px',
                        fontFamily: 'monospace', color: '#52c41a', borderColor: '#52c41a',
                      }}>
                        🕐 {fmtMs(step.stage_ms)}
                      </Tag>
                    )}
                    {/* 正在执行的阶段 */}
                    {isRunning && !isDone && (
                      <Tag style={{
                        margin: 0, fontSize: 10, lineHeight: '16px',
                        color: '#faad14', borderColor: '#faad14',
                      }}>
                        执行中...
                      </Tag>
                    )}
                  </div>
                  <Text type="secondary" style={{ fontSize: 11 }}>{step.message}</Text>
                </div>
              </div>
            );
          })}

          {/* 工具执行结果 */}
          {toolResults.length > 0 && (
            <div style={{
              marginTop: 6, paddingTop: 6, borderTop: '1px solid #e8e8e8',
              display: 'flex', flexWrap: 'wrap', gap: 4,
            }}>
              {toolResults.map((tr, i) => (
                <Tag key={i} color="orange" style={{ fontSize: 10 }}>
                  <ToolOutlined /> {tr.name} ({fmtMs(tr.elapsed_ms)})
                </Tag>
              ))}
            </div>
          )}

          {/* 总耗时 */}
          <div style={{
            marginTop: 6, paddingTop: 6, borderTop: '1px solid #e8e8e8',
            display: 'flex', alignItems: 'center', gap: 4,
          }}>
            <ClockCircleOutlined style={{ color: totalMs !== undefined ? '#52c41a' : '#faad14' }} />
            <Text strong style={{
              fontSize: 12,
              color: totalMs !== undefined ? '#52c41a' : '#faad14',
              fontFamily: 'monospace',
            }}>
              {totalMs !== undefined ? `总耗时 ${fmtMs(totalMs)}` : `已耗时 ${fmtMs(liveElapsed)}`}
            </Text>
          </div>
        </div>
      )}
    </div>
  );
};

const Assistant: React.FC = () => {
  const { currentTenant } = useOutletContext<LayoutContext>();
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [currentModel, setCurrentModel] = useState<ModelInfo | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // SSE 流式请求
  const onSend = useCallback(async () => {
    const text = input.trim();
    if (!text || sending) return;

    const userMsg: DisplayMessage = { role: 'user', content: text };
    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setSending(true);

    // 创建 AbortController
    const controller = new AbortController();
    abortControllerRef.current = controller;

    try {
      // 构建消息历史
      const history = messages
        .filter(m => m.role === 'user' || (m.role === 'assistant' && !m.isStreaming))
        .map(m => ({ role: m.role === 'status' ? 'assistant' : m.role, content: m.content }));
      history.push({ role: 'user', content: text });

      const token = localStorage.getItem('access_token');
      const response = await fetch(`/api/v1/tenants/${currentTenant}/chat`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ messages: history }),
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }

      const reader = response.body?.getReader();
      if (!reader) throw new Error('No reader');

      const decoder = new TextDecoder();
      let buffer = '';
      let currentContent = '';
      let currentTrace: TraceStep[] = [];
      let currentToolResults: ToolResult[] = [];
      let modelInfo: ModelInfo | null = null;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('event: ')) {
            continue;
          }

          if (line.startsWith('data: ')) {
            const dataStr = line.slice(6);
            
            // 回溯找 event 类型
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
                  const step: TraceStep = {
                    stage: data.stage || 'thinking',
                    message: data.message || '',
                    elapsed_ms: data.elapsed_ms,
                    stage_ms: data.stage_ms,
                    tool_name: data.tool_name,
                    tool_index: data.tool_index,
                    tool_total: data.tool_total,
                    timestamp: Date.now(),
                  };
                  currentTrace.push(step);

                  // 更新 status 消息
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    return [...filtered, {
                      role: 'status',
                      content: data.message,
                      modelInfo: modelInfo || undefined,
                      trace: [...currentTrace],
                      toolResults: [...currentToolResults],
                    }];
                  });
                  break;
                }

                case 'tool': {
                  currentToolResults.push({
                    name: data.name,
                    elapsed_ms: data.elapsed_ms || 0,
                  });
                  // 更新 status 消息
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    return [...filtered, {
                      role: 'status',
                      content: `✓ ${data.name} 完成`,
                      modelInfo: modelInfo || undefined,
                      trace: [...currentTrace],
                      toolResults: [...currentToolResults],
                    }];
                  });
                  break;
                }

                case 'delta':
                  // 流式文本片段
                  currentContent += data.content;
                  setMessages(prev => {
                    const filtered = prev.filter(m => m.role !== 'status');
                    const lastMsg = filtered[filtered.length - 1];
                    if (lastMsg && lastMsg.role === 'assistant' && lastMsg.isStreaming) {
                      const newMsgs = [...filtered];
                      newMsgs[newMsgs.length - 1] = {
                        role: 'assistant',
                        content: currentContent,
                        isStreaming: true,
                        modelInfo: modelInfo || undefined,
                        trace: [...currentTrace],
                        toolResults: [...currentToolResults],
                      };
                      return newMsgs;
                    } else {
                      return [...filtered, {
                        role: 'assistant',
                        content: currentContent,
                        isStreaming: true,
                        modelInfo: modelInfo || undefined,
                        trace: [...currentTrace],
                        toolResults: [...currentToolResults],
                      }];
                    }
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
                        ...lastMsg,
                        isStreaming: false,
                        totalMs,
                        modelInfo: modelInfo || undefined,
                        trace: currentTrace.length > 0 ? [...currentTrace] : lastMsg.trace,
                        toolResults: currentToolResults.length > 0 ? [...currentToolResults] : lastMsg.toolResults,
                      };
                      return newMsgs;
                    }
                    return filtered;
                  });
                  // 重置
                  currentContent = '';
                  currentTrace = [];
                  currentToolResults = [];
                  break;
                }

                case 'error':
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
  }, [input, sending, messages, currentTenant]);

  // 清空对话
  const onClear = () => {
    abortControllerRef.current?.abort();
    setMessages([]);
    setSending(false);
    setCurrentModel(null);
  };

  // 快捷命令
  const quickCommands = [
    { label: '👥 用户管理', cmd: '请帮我查看当前租户下的所有用户和角色' },
    { label: '🔐 角色分配', cmd: '请帮我给 admin@easp.com 分配管理员角色' },
    { label: '📊 租户概览', cmd: '请帮我全面了解当前租户的状态' },
    { label: '🔌 连接器诊断', cmd: '请帮我检查连接器和MCP工具的配置状态' },
    { label: '🔧 查看MCP工具', cmd: '请帮我查看当前租户下的MCP工具' },
  ];

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 120px)' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0, display: 'flex', alignItems: 'center', gap: 8 }}>
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
        <Button icon={<ClearOutlined />} onClick={onClear} disabled={messages.length === 0}>
          清空对话
        </Button>
      </div>

      {/* 消息区域 */}
      <Card 
        style={{ flex: 1, overflow: 'auto', marginBottom: 16, background: '#fafafa' }}
        bodyStyle={{ padding: 16 }}
      >
        {messages.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '40px 0' }}>
            <RobotOutlined style={{ fontSize: 48, color: '#d9d9d9', marginBottom: 16 }} />
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
                  <Avatar icon={<RobotOutlined />} style={{ backgroundColor: '#1677ff', marginRight: 8, flexShrink: 0 }} />
                )}
                {msg.role === 'status' && (
                  <Avatar icon={<LoadingOutlined />} style={{ backgroundColor: '#faad14', marginRight: 8, flexShrink: 0 }} />
                )}
                <div style={{ 
                  maxWidth: '75%',
                  padding: '12px 16px',
                  borderRadius: 12,
                  background: msg.role === 'user' ? '#1677ff' : msg.role === 'status' ? '#fffbe6' : '#fff',
                  color: msg.role === 'user' ? '#fff' : '#333',
                  boxShadow: '0 1px 2px rgba(0,0,0,0.1)',
                  border: msg.role === 'status' ? '1px solid #ffe58f' : 'none',
                }}>
                  {msg.role === 'status' ? (
                    <div>
                      {/* 气泡链路 */}
                      {msg.trace && msg.trace.length > 0 && (
                        <TraceTimeline
                          trace={msg.trace}
                          toolResults={msg.toolResults || []}
                          isStreaming={true}
                        />
                      )}
                      {/* 当前状态 */}
                      <Space>
                        <LoadingOutlined style={{ color: '#faad14' }} />
                        <Text style={{ color: '#8c6e00' }}>{msg.content}</Text>
                      </Space>
                    </div>
                  ) : msg.role === 'assistant' ? (
                    <>
                      {/* 气泡链路 - 流式和完成都显示 */}
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
                  <Avatar icon={<UserOutlined />} style={{ backgroundColor: '#87d068', marginLeft: 8, flexShrink: 0 }} />
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

      {/* 输入区域 */}
      <div style={{ display: 'flex', gap: 8 }}>
        <TextArea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="输入你的需求，例如：帮我查看用户列表、给 xxx 分配管理员角色..."
          autoSize={{ minRows: 1, maxRows: 4 }}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              onSend();
            }
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
        >
          发送
        </Button>
      </div>

      {/* 样式 */}
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
