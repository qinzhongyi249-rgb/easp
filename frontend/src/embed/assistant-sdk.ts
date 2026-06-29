type EASPAssistantOptions = {
  baseUrl?: string;
  tenantId?: string;
  tokenProvider: () => Promise<string>;
  pageContextProvider?: () => Record<string, unknown>;
  position?: 'right-bottom' | 'left-bottom';
  title?: string;
  welcome?: string;
  executionMode?: 'sandbox' | 'normal';
};

type ChatMessage = { role: 'user' | 'assistant'; content: string };
type ExecutionStep = { step: string; title: string; status: 'pending' | 'running' | 'done' | 'error'; result?: string };

const DEFAULT_WELCOME = '你好，我是 EASP AI 助手。';

function createStyles(): string {
  return `
    .easp-fab{position:fixed;width:56px;height:56px;border-radius:50%;border:0;background:#1677ff;color:#fff;box-shadow:0 8px 24px rgba(22,119,255,.35);font-size:22px;z-index:2147483647;cursor:pointer}
    .easp-panel{position:fixed;width:min(420px,calc(100vw - 24px));height:min(680px,calc(100vh - 24px));background:#fff;border:1px solid #e5e7eb;border-radius:18px;box-shadow:0 16px 48px rgba(15,23,42,.18);z-index:2147483647;display:none;overflow:hidden;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif}
    .easp-hd{height:52px;display:flex;align-items:center;justify-content:space-between;padding:0 14px;background:#1677ff;color:#fff;font-weight:600}.easp-close{border:0;background:transparent;color:#fff;font-size:20px;cursor:pointer}.easp-new{border:0;background:transparent;color:#fff;font-size:16px;cursor:pointer;margin-right:8px}
    .easp-msgs{flex:1;overflow:auto;padding:14px;background:#f8fafc}.msg{max-width:86%;margin:8px 0;padding:10px 12px;border-radius:14px;white-space:pre-wrap;line-height:1.5;font-size:14px}.user{margin-left:auto;background:#1677ff;color:#fff}.assistant{background:#fff;border:1px solid #e5e7eb;color:#111827}
    .step{max-width:90%;margin:8px 0;padding:8px 10px;border-radius:12px;background:#f1f5f9;border:1px solid #e2e8f0;font-size:13px}.step-title{display:flex;align-items:center;gap:6px;font-weight:500;color:#334155}.step-dot{width:8px;height:8px;border-radius:50%;background:#94a3b8}.step-dot.running{background:#f59e0b;animation:pulse 2s infinite}.step-dot.done{background:#10b981}.step-dot.error{background:#ef4444}.step-result{margin-top:6px;padding-top:6px;border-top:1px solid #e2e8f0;color:#475569;white-space:pre-wrap}@keyframes pulse{0%{opacity:1}50%{opacity:.5}100%{opacity:1}}
    .easp-ft{display:flex;gap:8px;padding:10px;border-top:1px solid #e5e7eb}.easp-input{flex:1;resize:none;border:1px solid #d1d5db;border-radius:10px;padding:9px;outline:none}.easp-send{border:0;border-radius:10px;background:#1677ff;color:#fff;padding:0 14px;cursor:pointer}
    @media(max-width:600px){.easp-panel{right:0!important;left:0!important;bottom:0!important;width:100vw;height:80vh;border-radius:18px 18px 0 0}.easp-fab{right:18px!important;left:auto!important}}
  `;
}

async function readSSE(res: Response, onText: (chunk: string) => void, onConversation?: (id: string) => void, onStep?: (step: ExecutionStep) => void) {
  const reader = res.body?.getReader();
  if (!reader) return;
  const decoder = new TextDecoder();
  let buffer = '';
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const events = buffer.split('\n\n');
    buffer = events.pop() || '';
    for (const event of events) {
      const eventTypeLine = event.split('\n').find((l) => l.startsWith('event:'));
      const line = event.split('\n').find((l) => l.startsWith('data:'));
      if (!line) continue;
      try {
        const data = JSON.parse(line.slice(5).trim());
        if (data.conversation_id && onConversation) onConversation(data.conversation_id);
        if (typeof data.content === 'string') onText(data.content);
        if (typeof data.delta === 'string') onText(data.delta);
        if (eventTypeLine) {
          const eventType = eventTypeLine.slice(6).trim();
          if (eventType === 'step' && onStep && data.step && data.title) {
            onStep({
              step: data.step,
              title: data.title,
              status: data.status || 'running',
              result: data.result
            });
          }
        }
      } catch { /* ignore non-json SSE */ }
    }
  }
}

export function init(options: EASPAssistantOptions) {
  if (!options?.tokenProvider) throw new Error('EASPAssistant.init: tokenProvider is required');
  const baseUrl = (options.baseUrl || '').replace(/\/$/, '');
  const pos = options.position === 'left-bottom' ? 'lb' : 'rb';
  const fabPos = options.position === 'left-bottom' ? 'fab-lb' : 'fab-rb';
  const host = document.createElement('div');
  const shadow = host.attachShadow({ mode: 'open' });
  shadow.innerHTML = `<style>${createStyles()}</style><button class="easp-fab ${fabPos}" aria-label="EASP AI 助手">AI</button><section class="easp-panel ${pos}"><header class="easp-hd"><span>${options.title || 'EASP AI 助手'}</span><button class="easp-close">×</button></header><main class="easp-msgs"></main><footer class="easp-ft"><textarea class="easp-input" rows="2" placeholder="输入问题..."></textarea><button class="easp-send">发送</button></footer></section>`;
  document.body.appendChild(host);

  const fab = shadow.querySelector<HTMLButtonElement>('.easp-fab')!;
  const panel = shadow.querySelector<HTMLElement>('.easp-panel')!;
  const close = shadow.querySelector<HTMLButtonElement>('.easp-close')!;
  const msgs = shadow.querySelector<HTMLElement>('.easp-msgs')!;
  const input = shadow.querySelector<HTMLTextAreaElement>('.easp-input')!;
  const send = shadow.querySelector<HTMLButtonElement>('.easp-send')!;
  let conversationId = localStorage.getItem('easp_embed_conversation_id') || '';
  let token = '';

  const append = (role: ChatMessage['role'], content: string) => {
    let el = msgs.lastElementChild as HTMLElement | null;
    if (!el || !el.classList.contains(role) || role === 'user') {
      el = document.createElement('div');
      el.className = `msg ${role}`;
      msgs.appendChild(el);
    }
    el.textContent = content;
    msgs.scrollTop = msgs.scrollHeight;
  };

  const appendStep = (step: ExecutionStep) => {
    const el = document.createElement('div');
    el.className = `step step-${step.status}`;
    el.innerHTML = `
      <div class="step-title">
        <span class="step-dot ${step.status}"></span>
        <span>${step.title}</span>
      </div>
      ${step.result ? `<div class="step-result">${escapeHtml(step.result)}</div>` : ''}
    `;
    msgs.appendChild(el);
    msgs.scrollTop = msgs.scrollHeight;
    // 找不到 escapeHtml 这里定义
  };

  function escapeHtml(text: string): string {
    return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#039;');
  }

  append('assistant', options.welcome || DEFAULT_WELCOME);
  fab.onclick = () => panel.classList.toggle('open');
  close.onclick = () => panel.classList.remove('open');

  const submit = async () => {
    const text = input.value.trim();
    if (!text) return;
    input.value = '';
    append('user', text);
    let assistantText = '';
    append('assistant', '');
    try {
      token = token || await options.tokenProvider();
      const payload = {
        conversation_id: conversationId || undefined,
        messages: [{ role: 'user', content: text }],
        page_context: options.pageContextProvider?.() || { url: location.href, title: document.title },
        execution_mode: options.executionMode,
      };
      const res = await fetch(`${baseUrl}/embed/v1/assistant/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'easp-api-token': token },
        body: JSON.stringify(payload),
      });
      if (res.status === 401) {
        token = await options.tokenProvider();
        return submit();
      }
      if (!res.ok) throw new Error(await res.text());
      await readSSE(res, (chunk) => { assistantText += chunk; append('assistant', assistantText); }, (id) => { conversationId = id; localStorage.setItem('easp_embed_conversation_id', id); }, appendStep);
    } catch (err) {
      append('assistant', `请求失败：${err instanceof Error ? err.message : String(err)}`);
    }
  };

  send.onclick = submit;
  input.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); } });
  return { open: () => panel.classList.add('open'), close: () => panel.classList.remove('open'), destroy: () => host.remove() };
}

const api = { init };
// @ts-expect-error browser global
window.EASPAssistant = api;
export default api;
