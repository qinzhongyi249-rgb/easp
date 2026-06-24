(function () {
  function styles() {
    return '.easp-fab{position:fixed;width:56px;height:56px;border-radius:50%;border:0;background:#1677ff;color:#fff;box-shadow:0 8px 24px rgba(22,119,255,.35);font-size:22px;z-index:2147483647;cursor:pointer}.easp-panel{position:fixed;width:min(420px,calc(100vw - 24px));height:min(680px,calc(100vh - 24px));background:#fff;border:1px solid #e5e7eb;border-radius:18px;box-shadow:0 16px 48px rgba(15,23,42,.18);z-index:2147483647;display:none;overflow:hidden;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif}.easp-panel.open{display:flex;flex-direction:column}.rb{right:20px;bottom:88px}.lb{left:20px;bottom:88px}.fab-rb{right:24px;bottom:24px}.fab-lb{left:24px;bottom:24px}.easp-hd{height:52px;display:flex;align-items:center;justify-content:space-between;padding:0 14px;background:#1677ff;color:#fff;font-weight:600}.easp-close{border:0;background:transparent;color:#fff;font-size:20px;cursor:pointer}.easp-msgs{flex:1;overflow:auto;padding:14px;background:#f8fafc}.msg{max-width:86%;margin:8px 0;padding:10px 12px;border-radius:14px;white-space:pre-wrap;line-height:1.5;font-size:14px}.user{margin-left:auto;background:#1677ff;color:#fff}.assistant{background:#fff;border:1px solid #e5e7eb;color:#111827}.easp-ft{display:flex;gap:8px;padding:10px;border-top:1px solid #e5e7eb}.easp-input{flex:1;resize:none;border:1px solid #d1d5db;border-radius:10px;padding:9px;outline:none}.easp-send{border:0;border-radius:10px;background:#1677ff;color:#fff;padding:0 14px;cursor:pointer}@media(max-width:600px){.easp-panel{right:0!important;left:0!important;bottom:0!important;width:100vw;height:80vh;border-radius:18px 18px 0 0}.easp-fab{right:18px!important;left:auto!important}}';
  }
  async function readSSE(res, onText, onConversation) {
    var reader = res.body && res.body.getReader();
    if (!reader) return;
    var decoder = new TextDecoder();
    var buffer = '';
    while (true) {
      var r = await reader.read();
      if (r.done) break;
      buffer += decoder.decode(r.value, { stream: true });
      var events = buffer.split('\n\n');
      buffer = events.pop() || '';
      for (var i = 0; i < events.length; i++) {
        var line = events[i].split('\n').find(function (l) { return l.indexOf('data:') === 0; });
        if (!line) continue;
        try {
          var data = JSON.parse(line.slice(5).trim());
          if (data.conversation_id && onConversation) onConversation(data.conversation_id);
          if (typeof data.content === 'string') onText(data.content);
          if (typeof data.delta === 'string') onText(data.delta);
        } catch (e) {}
      }
    }
  }
  function init(options) {
    if (!options || !options.tokenProvider) throw new Error('EASPAssistant.init: tokenProvider is required');
    var baseUrl = (options.baseUrl || '').replace(/\/$/, '');
    var pos = options.position === 'left-bottom' ? 'lb' : 'rb';
    var fabPos = options.position === 'left-bottom' ? 'fab-lb' : 'fab-rb';
    var host = document.createElement('div');
    var shadow = host.attachShadow({ mode: 'open' });
    shadow.innerHTML = '<style>' + styles() + '</style><button class="easp-fab ' + fabPos + '" aria-label="EASP AI 助手">AI</button><section class="easp-panel ' + pos + '"><header class="easp-hd"><span>' + (options.title || 'EASP AI 助手') + '</span><button class="easp-close">×</button></header><main class="easp-msgs"></main><footer class="easp-ft"><textarea class="easp-input" rows="2" placeholder="输入问题..."></textarea><button class="easp-send">发送</button></footer></section>';
    document.body.appendChild(host);
    var fab = shadow.querySelector('.easp-fab');
    var panel = shadow.querySelector('.easp-panel');
    var close = shadow.querySelector('.easp-close');
    var msgs = shadow.querySelector('.easp-msgs');
    var input = shadow.querySelector('.easp-input');
    var send = shadow.querySelector('.easp-send');
    var conversationId = localStorage.getItem('easp_embed_conversation_id') || '';
    var token = '';
    function append(role, content) {
      var el = msgs.lastElementChild;
      if (!el || !el.classList.contains(role) || role === 'user') {
        el = document.createElement('div');
        el.className = 'msg ' + role;
        msgs.appendChild(el);
      }
      el.textContent = content;
      msgs.scrollTop = msgs.scrollHeight;
    }
    append('assistant', options.welcome || '你好，我是 EASP AI 助手。');
    fab.onclick = function () { panel.classList.toggle('open'); };
    close.onclick = function () { panel.classList.remove('open'); };
    async function submit() {
      var text = input.value.trim();
      if (!text) return;
      input.value = '';
      append('user', text);
      var assistantText = '';
      append('assistant', '');
      try {
        token = token || await options.tokenProvider();
        var payload = { conversation_id: conversationId || undefined, messages: [{ role: 'user', content: text }], page_context: (options.pageContextProvider && options.pageContextProvider()) || { url: location.href, title: document.title } };
        var res = await fetch(baseUrl + '/embed/v1/assistant/chat', { method: 'POST', headers: { 'Content-Type': 'application/json', 'easp-api-token': token }, body: JSON.stringify(payload) });
        if (res.status === 401) { token = await options.tokenProvider(); res = await fetch(baseUrl + '/embed/v1/assistant/chat', { method: 'POST', headers: { 'Content-Type': 'application/json', 'easp-api-token': token }, body: JSON.stringify(payload) }); }
        if (!res.ok) throw new Error(await res.text());
        await readSSE(res, function (chunk) { assistantText += chunk; append('assistant', assistantText); }, function (id) { conversationId = id; localStorage.setItem('easp_embed_conversation_id', id); });
      } catch (err) {
        append('assistant', '请求失败：' + (err && err.message ? err.message : String(err)));
      }
    }
    send.onclick = submit;
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); } });
    return { open: function () { panel.classList.add('open'); }, close: function () { panel.classList.remove('open'); }, destroy: function () { host.remove(); } };
  }
  window.EASPAssistant = { init: init };
})();
