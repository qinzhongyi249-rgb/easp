import React, { useState, useRef, useEffect, useCallback } from 'react';
import { Input, Button, Typography, Space, Avatar, Tag, Tooltip, Badge } from 'antd';
import {
  SendOutlined, RobotOutlined, UserOutlined, PlusOutlined,
  LoadingOutlined, CheckCircleOutlined, CloseOutlined,
  CustomerServiceOutlined, ClockCircleOutlined, ThunderboltOutlined,
  ToolOutlined, BranchesOutlined, DownOutlined,
} from '@ant-design/icons';
import { MarkdownRenderer, MARKDOWN_CSS } from '../utils/markdown';
import {
  buildAssistantPageContext,
  clearAssistantConversationId,
  loadAssistantConversationId,
  saveAssistantConversationId,
} from '../utils/assistantContext';

const { Text } = Typography;
const { TextArea } = Input;

interface ModelInfo {
  model: string;
  display_name: string;
  provider: string;
}

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

interface FloatingAssistantProps {
  tenantId: string;
  userId?: string;
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
    const nextMsg: DisplayMessage = {
      role: 'assistant',
      content,
      isStreaming: true,
      modelInfo: modelInfo || undefined,
      trace: traceSteps.length > 0 ? traceSteps : undefined,
      toolResults: toolResults.length > 0 ? toolResults : undefined,
    };
    const filtered = prev.filter(m => !(m.role === 'status' && m.isStreaming));
    const streamingIndex = filtered.findLastIndex(m => m.role === 'assistant' && m.isStreaming);
    if (streamingIndex >= 0) {
      const newMsgs = [...filtered];
      newMsgs[streamingIndex] = { ...newMsgs[streamingIndex], ...nextMsg };
      return newMsgs;
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

  if (uniqueSteps.length === 0 && toolResults.length === 0) return null;

  const firstTimestamp = uniqueSteps.length > 0 && uniqueSteps[0].timestamp ? uniqueSteps[0].timestamp : Date.now();
  const liveElapsed = totalMs ?? Math.max(0, Date.now() - firstTimestamp);
  const lastStep = uniqueSteps[uniqueSteps.length - 1];
  const summaryText = lastStep?.stage === 'generating' ? '生成中...' :
    lastStep?.stage === 'tool_calling' ? `执行工具 ${lastStep.tool_index || 1}/${lastStep.tool_total || 1}` :
    lastStep?.message || '处理中...';

  return (
    <div style={{ marginBottom: 8 }}>
      <div
        style={{ display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer', padding: '2px 0' }}
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
            const running = isStreaming && i === uniqueSteps.length - 1 && step.stage_ms === undefined;
            return (
              <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '4px 0' }}>
                <span style={{ color: config.color }}>{config.icon}</span>
                <Text style={{ fontSize: 12, flex: 1 }}>
                  {step.tool_name ? `${config.label}: ${step.tool_name}` : config.label}
                </Text>
                <Text type="secondary" style={{ fontSize: 11, fontFamily: 'monospace' }}>
                  {step.stage_ms !== undefined ? fmtMs(step.stage_ms) : running ? '执行中...' : ''}
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

// localStorage 工具函数
const getChatKey = (userId: string, tenantId: string) =>
  `easp_chat_${userId}_${tenantId}`;

const loadMessages = (userId: string, tenantId: string): DisplayMessage[] => {
  try {
    const key = getChatKey(userId, tenantId);
    const raw = localStorage.getItem(key);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return parsed;
    }
  } catch { /* ignore */ }
  return [];
};

const saveMessages = (userId: string, tenantId: string, messages: DisplayMessage[]) => {
  try {
    const key = getChatKey(userId, tenantId);
    localStorage.setItem(key, JSON.stringify(messages));
  } catch { /* ignore */ }
};

// 安全边界
const BOUNDARY_MARGIN = 20;
const clamp = (val: number, min: number, max: number) => Math.max(min, Math.min(max, val));

// 初始位置：右下角
const getInitialPos = (): { x: number; y: number } => {
  try {
    const raw = localStorage.getItem('easp_fab_pos');
    if (raw) {
      const p = JSON.parse(raw);
      if (typeof p.x === 'number' && typeof p.y === 'number') {
        return { x: clamp(p.x, BOUNDARY_MARGIN, window.innerWidth - 76), y: clamp(p.y, BOUNDARY_MARGIN, window.innerHeight - 76) };
      }
    }
  } catch { /* ignore */ }
  return { x: window.innerWidth - 80, y: window.innerHeight - 80 };
};

const FloatingAssistant: React.FC<FloatingAssistantProps> = ({ tenantId, userId }) => {
  // —— 聊天状态 ——
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<DisplayMessage[]>([]);
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const [currentModel, setCurrentModel] = useState<ModelInfo | null>(null);
  const [unread, setUnread] = useState(0);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);

  // —— 拖拽状态 ——
  const [pos, setPos] = useState(getInitialPos);
  const [isDragging, setIsDragging] = useState(false);
  const draggingRef = useRef(false);
  const dragMovedRef = useRef(false);
  const startRef = useRef({ x: 0, y: 0, posX: 0, posY: 0 });
  const animRef = useRef<number | null>(null);

  // 加载历史消息
  useEffect(() => {
    if (userId && tenantId) {
      const saved = loadMessages(userId, tenantId);
      if (saved.length > 0) setMessages(saved);
    }
  }, [userId, tenantId]);

  // 自动滚动
  useEffect(() => {
    if (open) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
      setUnread(0);
    }
  }, [messages, open]);

  // 保存消息（流式结束后）
  useEffect(() => {
    if (!userId || !tenantId) return;
    if (sending) return; // 流式进行中不保存
    if (messages.length === 0) return;
    const hasStreaming = messages.some(m => m.isStreaming);
    if (hasStreaming) return;
    const clean = messages.filter(m => m.role !== 'status');
    if (clean.length > 0) saveMessages(userId, tenantId, clean);
  }, [messages, sending, userId, tenantId]);

  // 保存按钮位置
  useEffect(() => {
    try { localStorage.setItem('easp_fab_pos', JSON.stringify(pos)); } catch { /* ignore */ }
  }, [pos]);

  // 窗口 resize 时修正位置
  useEffect(() => {
    const onResize = () => {
      setPos(p => ({
        x: clamp(p.x, BOUNDARY_MARGIN, window.innerWidth - 76),
        y: clamp(p.y, BOUNDARY_MARGIN, window.innerHeight - 76),
      }));
    };
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  // 吸边动画
  const snapToEdge = useCallback((x: number, y: number) => {
    if (animRef.current) cancelAnimationFrame(animRef.current);

    const W = window.innerWidth, H = window.innerHeight;
    const toLeft = x < W / 2;
    const targetX = toLeft ? BOUNDARY_MARGIN : W - 76;
    const targetY = clamp(y, BOUNDARY_MARGIN, H - 76);

    let currentX = x, currentY = y;
    const step = () => {
      const dx = targetX - currentX, dy = targetY - currentY;
      if (Math.abs(dx) < 0.5 && Math.abs(dy) < 0.5) {
        setPos({ x: targetX, y: targetY });
        animRef.current = null;
        return;
      }
      currentX += dx * 0.25;
      currentY += dy * 0.25;
      setPos({ x: Math.round(currentX), y: Math.round(currentY) });
      animRef.current = requestAnimationFrame(step);
    };
    animRef.current = requestAnimationFrame(step);
  }, []);

  // —— 鼠标拖拽（PC）——
  useEffect(() => {
    const onMouseMove = (e: MouseEvent) => {
      if (!draggingRef.current) return;
      const dx = e.clientX - startRef.current.x;
      const dy = e.clientY - startRef.current.y;
      if (Math.abs(dx) > 2 || Math.abs(dy) > 2) dragMovedRef.current = true;
      const nx = clamp(startRef.current.posX + dx, BOUNDARY_MARGIN, window.innerWidth - 76);
      const ny = clamp(startRef.current.posY + dy, BOUNDARY_MARGIN, window.innerHeight - 76);
      setPos({ x: nx, y: ny });
    };
    const onMouseUp = (e: MouseEvent) => {
      if (!draggingRef.current) return;
      draggingRef.current = false;
      setIsDragging(false);
      if (dragMovedRef.current) {
        e.preventDefault();
        snapToEdge(e.clientX, e.clientY);
      }
    };
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
    return () => {
      window.removeEventListener('mousemove', onMouseMove);
      window.removeEventListener('mouseup', onMouseUp);
    };
  }, [snapToEdge]);

  // —— 触摸拖拽（移动端）——
  useEffect(() => {
    const onTouchMove = (e: TouchEvent) => {
      if (!draggingRef.current) return;
      e.preventDefault(); // 阻止页面滚动
      const t = e.touches[0];
      const dx = t.clientX - startRef.current.x;
      const dy = t.clientY - startRef.current.y;
      if (Math.abs(dx) > 2 || Math.abs(dy) > 2) dragMovedRef.current = true;
      const nx = clamp(startRef.current.posX + dx, BOUNDARY_MARGIN, window.innerWidth - 76);
      const ny = clamp(startRef.current.posY + dy, BOUNDARY_MARGIN, window.innerHeight - 76);
      setPos({ x: nx, y: ny });
    };
    const onTouchEnd = (e: TouchEvent) => {
      if (!draggingRef.current) return;
      draggingRef.current = false;
      setIsDragging(false);
      if (dragMovedRef.current) {
        e.preventDefault();
        const t = e.changedTouches[0];
        snapToEdge(t.clientX, t.clientY);
      }
    };
    window.addEventListener('touchmove', onTouchMove, { passive: false });
    window.addEventListener('touchend', onTouchEnd);
    return () => {
      window.removeEventListener('touchmove', onTouchMove);
      window.removeEventListener('touchend', onTouchEnd);
    };
  }, [snapToEdge]);

  // 清理动画帧
  useEffect(() => () => { if (animRef.current) cancelAnimationFrame(animRef.current); }, []);

  const onPointerDown = (e: React.MouseEvent | React.TouchEvent) => {
    if (animRef.current) { cancelAnimationFrame(animRef.current); animRef.current = null; }
    const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
    const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
    startRef.current = { x: clientX, y: clientY, posX: pos.x, posY: pos.y };
    draggingRef.current = true;
    dragMovedRef.current = false;
    setIsDragging(true);
  };

  const onPointerClick = () => {
    if (!dragMovedRef.current && !open) setOpen(true);
  };

  // —— SSE 流式请求 ——
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
      const token = localStorage.getItem('access_token');
      const conversationId = loadAssistantConversationId('floating_assistant', userId, tenantId);
      const response = await fetch(`/api/v1/tenants/${tenantId}/chat`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({
          conversation_id: conversationId || undefined,
          page_context: buildAssistantPageContext('floating_assistant', tenantId, userId),
          // 服务端会按 conversation_id 拼接历史；前端只传本轮输入，避免历史重复注入。
          messages: [{ role: 'user', content: text }],
        }),
        signal: controller.signal,
      });

      if (!response.ok) throw new Error(`HTTP ${response.status}`);

      const reader = response.body?.getReader();
      if (!reader) throw new Error('No reader');

      const decoder = new TextDecoder();
      let buffer = '';
      let currentEvent = '';
      let traceSteps: TraceStep[] = [];
      let toolResults: ToolResult[] = [];
      let modelInfo: ModelInfo | null = null;
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

      const appendStatus = (content: string, step?: TraceStep, totalMs?: number) => {
        setMessages(prev => {
          const nextStatus: DisplayMessage = {
            role: 'status',
            content,
            isStreaming: true,
            // 过程气泡只保留一个；trace 存完整轨迹，折叠态只滚动展示最新阶段。
            trace: traceSteps.length > 0 ? traceSteps : step ? [step] : undefined,
            toolResults: toolResults.length > 0 ? toolResults : undefined,
            totalMs,
          };
          const statusIndex = prev.findLastIndex(m => m.role === 'status' && m.isStreaming);
          if (statusIndex >= 0) {
            const newMsgs = [...prev];
            newMsgs[statusIndex] = { ...newMsgs[statusIndex], ...nextStatus };
            return newMsgs;
          }
          return [...prev, nextStatus];
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
          const dataStr = line.slice(6);

          try {
            const data = JSON.parse(dataStr);
            const eventType = currentEvent || data.type || 'message';
            currentEvent = '';

            switch (eventType) {
              case 'conversation':
                if (data.conversation_id) {
                  saveAssistantConversationId('floating_assistant', userId, tenantId, data.conversation_id);
                }
                break;
              case 'model_info':
              case 'model':
                modelInfo = data;
                setCurrentModel(data);
                break;

              case 'status':
              case 'heartbeat':
              case 'trace': {
                const step: TraceStep = { ...data, timestamp: data.timestamp || Date.now() };
                traceSteps = [...traceSteps, step];
                appendStatus(data.message || '处理中...', step, data.total_ms ?? data.elapsed_ms);
                break;
              }

              case 'tool':
              case 'tool_result':
                toolResults = [...toolResults, { name: data.name, elapsed_ms: data.elapsed_ms }];
                appendStatus(`✓ ${data.name} 完成`, {
                  stage: 'tool_calling',
                  message: `✓ ${data.name} 完成`,
                  tool_name: data.name,
                  stage_ms: data.elapsed_ms,
                  elapsed_ms: data.elapsed_ms,
                  timestamp: Date.now(),
                }, data.total_ms);
                break;

              case 'delta': {
                const piece = data.content || '';
                enqueueDelta(piece);
                break;
              }

              case 'done': {
                flushTypewriter();
                const totalMs = data?.total_ms;
                const finalContent = displayedContent || '处理已完成，但模型没有返回可展示内容。请稍后重试或简化问题。';
                setMessages(prev => {
                  const nextMsg: DisplayMessage = {
                    role: 'assistant',
                    content: finalContent,
                    isStreaming: false,
                    totalMs,
                    modelInfo: modelInfo || undefined,
                    trace: traceSteps.length > 0 ? traceSteps : undefined,
                    toolResults: toolResults.length > 0 ? toolResults : undefined,
                  };
                  const filtered = prev.filter(m => !(m.role === 'status' && m.isStreaming));
                  const streamingIndex = filtered.findLastIndex(m => m.role === 'assistant' && m.isStreaming);
                  if (streamingIndex >= 0) {
                    const newMsgs = [...filtered];
                    newMsgs[streamingIndex] = { ...newMsgs[streamingIndex], ...nextMsg };
                    return newMsgs;
                  }
                  return [...filtered, nextMsg];
                });
                if (!open) setUnread(u => u + 1);
                break;
              }

              case 'error':
                flushTypewriter();
                setMessages(prev => {
                  const filtered = prev.filter(m => !(m.role === 'status' && m.isStreaming));
                  return [...filtered, {
                    role: 'assistant',
                    content: `❌ ${data.message}`,
                    trace: traceSteps.length > 0 ? traceSteps : undefined,
                    toolResults: toolResults.length > 0 ? toolResults : undefined,
                  }];
                });
                break;
            }
          } catch { /* ignore parse errors */ }
        }
      }
    } catch (err: unknown) {
      if ((err as Error).name === 'AbortError') return;
      const e = err as { message?: string };
      setMessages(prev => {
        const filtered = prev.filter(m => !(m.role === 'status' && m.isStreaming));
        return [...filtered, { role: 'assistant', content: `❌ 请求失败: ${e.message || '未知错误'}` }];
      });
    } finally {
      setSending(false);
      abortControllerRef.current = null;
    }
  }, [input, sending, messages, tenantId, userId, open]);

  // 开启新对话
  const onNewChat = useCallback(() => {
    abortControllerRef.current?.abort();
    setMessages([]);
    setSending(false);
    setCurrentModel(null);
    setUnread(0);
    if (userId && tenantId) {
      try { localStorage.removeItem(getChatKey(userId, tenantId)); } catch { /* ignore */ }
    }
    clearAssistantConversationId('floating_assistant', userId, tenantId);
  }, [userId, tenantId]);

  return (
    <>
      {/* 浮动按钮 — 面板打开时隐藏 */}
      {!open && (
      <Badge count={unread} size="small" offset={[-6, 0]}>
        <div
          onMouseDown={onPointerDown}
          onTouchStart={onPointerDown}
          onClick={onPointerClick}
          style={{
            position: 'fixed',
            left: pos.x,
            top: pos.y,
            zIndex: 1000,
            width: 56, height: 56, borderRadius: '50%',
            background: 'linear-gradient(135deg, #1677ff, #4096ff)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            cursor: isDragging ? 'grabbing' : 'grab',
            boxShadow: isDragging
              ? '0 8px 32px rgba(22,119,255,0.5)'
              : '0 4px 16px rgba(22,119,255,0.4)',
            transition: isDragging ? 'none' : 'box-shadow 0.3s',
            touchAction: 'none',
            userSelect: 'none',
            WebkitUserSelect: 'none',
          }}
        >
          <CustomerServiceOutlined style={{ fontSize: 26, color: '#fff', pointerEvents: 'none' }} />
        </div>
      </Badge>
      )}

      {/* 聊天弹窗 */}
      {open && (
        <div style={{
          position: 'fixed',
          right: 0, top: 0,
          width: 'min(420px, 100vw)',
          height: '100vh',
          zIndex: 1001,
          background: '#fff',
          boxShadow: '-4px 0 24px rgba(0,0,0,0.12)',
          display: 'flex', flexDirection: 'column',
          animation: 'easp-slide-in 0.25s ease-out',
        }}>
          {/* Header */}
          <div style={{
            padding: '12px 16px', borderBottom: '1px solid #f0f0f0',
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            background: '#fff', flexShrink: 0,
          }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <RobotOutlined style={{ color: '#1677ff' }} />
              <span style={{ fontWeight: 600 }}>AI 助手</span>
              {currentModel && (
                <Tooltip title={`${currentModel.model} | ${currentModel.provider}`}>
                  <Tag color="blue" style={{ fontSize: 11, cursor: 'pointer', margin: 0 }}>
                    {currentModel.display_name || currentModel.model}
                  </Tag>
                </Tooltip>
              )}
              {sending && <Tag icon={<LoadingOutlined />} color="processing" style={{ margin: 0, fontSize: 11 }}>思考中</Tag>}
            </div>
            <Space size={4}>
              <Tooltip title="新对话">
                <Button
                  type="text"
                  icon={<PlusOutlined />}
                  size="small"
                  onClick={onNewChat}
                  disabled={messages.length === 0 && !sending}
                />
              </Tooltip>
              <Button
                type="text"
                icon={<CloseOutlined />}
                size="small"
                onClick={() => setOpen(false)}
              />
            </Space>
          </div>

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
                        <div>
                          {msg.trace && msg.trace.length > 0 && (
                            <TraceTimeline
                              trace={msg.trace}
                              toolResults={msg.toolResults || []}
                              isStreaming={true}
                            />
                          )}
                          <Space size={6}>
                            <LoadingOutlined style={{ color: '#faad14', fontSize: 12 }} />
                            <Text style={{ color: '#8c6e00', fontSize: 12 }}>{msg.content}</Text>
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
                          <MarkdownRenderer content={msg.content} compact />
                          {msg.isStreaming && msg.content && <span style={{ color: '#1677ff', fontWeight: 'bold', animation: 'blink 0.8s infinite' }}>▊</span>}
                          {!msg.isStreaming && msg.totalMs !== undefined && (
                            <div style={{ marginTop: 4, fontSize: 11, color: '#999', fontFamily: 'monospace' }}>
                              总耗时 {fmtMs(msg.totalMs)}
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
          <div style={{ padding: '12px 16px', borderTop: '1px solid #f0f0f0', background: '#fff', flexShrink: 0 }}>
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
        </div>
      )}

      {/* 遮罩层 */}
      {open && (
        <div
          onClick={() => setOpen(false)}
          style={{
            position: 'fixed', inset: 0,
            background: 'rgba(0,0,0,0.15)',
            zIndex: 1000,
          }}
        />
      )}

      <style>{`
        @keyframes blink {
          0%, 100% { opacity: 1; }
          50% { opacity: 0; }
        }
        @keyframes easp-slide-in {
          from { transform: translateX(100%); }
          to { transform: translateX(0); }
        }
        ${MARKDOWN_CSS}
      `}</style>
    </>
  );
};

export default FloatingAssistant;
