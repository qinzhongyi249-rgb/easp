(function () {
  function styles() {
    return '.easp-fab{position:fixed;width:56px;height:56px;border-radius:50%;border:0;background:#1677ff;color:#fff;box-shadow:0 8px 24px rgba(22,119,255,.35);font-size:22px;z-index:2147483647;cursor:pointer;user-select:none}.easp-fab.dragging{cursor:grabbing}.easp-fab img,.easp-fab svg{width:100%;height:100%;border-radius:50%;object-fit:cover}.easp-panel{position:fixed;width:min(420px,calc(100vw - 24px));height:min(680px,calc(100vh - 24px));background:#fff;border:1px solid #e5e7eb;border-radius:18px;box-shadow:0 16px 48px rgba(15,23,42,.18);z-index:2147483647;display:none;flex-direction:column;overflow:hidden;font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif}.easp-panel.open{display:flex}.rb{right:20px;bottom:88px}.lb{left:20px;bottom:88px}.fab-rb{right:24px;bottom:24px}.fab-lb{left:24px;bottom:24px}.easp-hd{height:52px;display:flex;align-items:center;justify-content:space-between;padding:0 14px;background:#1677ff;color:#fff;font-weight:600}.easp-hd .easp-actions{display:flex;gap:8px;align-items:center}.easp-new{color:#fff;background:rgba(255,255,255,0.2);border:0;font-size:18px;padding:0 8px;border-radius:6px;cursor:pointer}.easp-close{border:0;background:transparent;color:#fff;font-size:20px;cursor:pointer}.easp-msgs{flex:1;overflow:auto;padding:14px;background:#f8fafc}.msg{max-width:86%;margin:8px 0;padding:10px 12px;border-radius:14px;white-space:pre-wrap;line-height:1.5;font-size:14px}.msg .msg-meta{font-size:12px;color:#999;margin-bottom:4px}.msg.user{margin-left:auto;background:#1677ff;color:#fff}.msg.assistant{background:#fff;border:1px solid #e5e7eb;color:#111827}.easp-ft{display:flex;gap:8px;padding:10px;border-top:1px solid #e5e7eb}.easp-input{flex:1;resize:none;border:1px solid #d1d5db;border-radius:10px;padding:9px;outline:none}.easp-send{border:0;border-radius:10px;background:#1677ff;color:#fff;padding:0 14px;cursor:pointer}@media(max-width:600px){.easp-panel{right:0!important;left:0!important;bottom:0!important;width:100vw;height:80vh;border-radius:18px 18px 0 0}.easp-fab{right:18px!important;left:auto!important}}';
  }
  async function readSSE(res, onText, onStep, onConversation) {
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
          if (data.step && onStep) onStep(data.step);
        } catch (e) {}
      }
    }
  }
  function init(options) {
    if (!options || !options.tokenProvider) throw new Error('EASPAssistant.init: tokenProvider is required');
    console.log('[EASPAssistant] init with options:', options);
    var baseUrl = (options.baseUrl || '').replace(/\/$/, '');
    var pos = options.position === 'left-bottom' ? 'lb' : 'rb';
    var fabPos = options.position === 'left-bottom' ? 'fab-lb' : 'fab-rb';
    var host = document.createElement('div');
    var shadow = host.attachShadow({ mode: 'open' });
    var fabHtml = '<button class="easp-fab ' + fabPos + '" aria-label="' + (options.title || 'EASP AI 助手') + '">';
    if (options.iconUrl) {
      fabHtml += '<img src="' + options.iconUrl + '" alt=""/>';
    } else if (options.iconSvg) {
      fabHtml += options.iconSvg;
    } else {
      fabHtml += 'AI';
    }
    fabHtml += '</button><section class="easp-panel ' + pos + '"><header class="easp-hd"><span>' + (options.title || 'EASP AI 助手') + '</span><div class="easp-actions"><button class="easp-new" title="新建对话">+</button><button class="easp-close">×</button></div></header><main class="easp-msgs"></main><footer class="easp-ft"><textarea class="easp-input" rows="2" placeholder="输入问题..."></textarea><button class="easp-send">发送</button></footer></section>';
    shadow.innerHTML = '<style>' + styles() + '</style>' + fabHtml;
    document.body.appendChild(host);
    var fab = shadow.querySelector('.easp-fab');
    var panel = shadow.querySelector('.easp-panel');
    var close = shadow.querySelector('.easp-close');
    var newBtn = shadow.querySelector('.easp-new');
    var msgs = shadow.querySelector('.easp-msgs');
    var input = shadow.querySelector('.easp-input');
    var send = shadow.querySelector('.easp-send');
    newBtn.onclick = newConversation;
    var conversationId = localStorage.getItem('easp_embed_conversation_id') || '';
    var token = '';

    // 新建对话
    function newConversation() {
      conversationId = '';
      msgs.innerHTML = '';
      localStorage.removeItem('easp_embed_conversation_id');
      append('assistant', options.welcome || '你好，我是 EASP AI 助手。');
    }

    // 获取执行模式，默认 normal 真实执行，可选 sandbox 只规划不执行
    var executionMode = options.executionMode || 'normal';

    // 拖拽支持
    var isDragging = false;
    var startX, startY, initialLeft, initialTop;

    // 保存位置到 localStorage
    function savePosition(left, bottom) {
      try {
        localStorage.setItem('easp_fab_position', JSON.stringify({ left: left, bottom: bottom }));
      } catch(e) {}
    }
    function loadPosition() {
      try {
        var pos = JSON.parse(localStorage.getItem('easp_fab_position'));
        if (pos && pos.left != null && pos.bottom != null) {
          fab.style.left = pos.left + 'px';
          fab.style.bottom = pos.bottom + 'px';
          fab.style.right = 'auto';
        }
      } catch(e) {}
    }
    loadPosition();

    fab.addEventListener('mousedown', function(e) {
      if (panel.classList.contains('open') && e.target.closest('.easp-panel')) return;
      isDragging = true;
      fab.classList.add('dragging');
      startX = e.clientX;
      startY = e.clientY;
      var rect = fab.getBoundingClientRect();
      initialLeft = rect.left;
      initialTop = window.innerHeight - rect.bottom;
      document.body.style.userSelect = 'none';
    });

    document.addEventListener('mousemove', function(e) {
      if (!isDragging) return;
      var dx = e.clientX - startX;
      var dy = e.clientY - startY;
      var left = initialLeft + dx;
      var bottom = initialTop + dy;
      // 边界限制
      if (left < 0) left = 0;
      if (left + fab.offsetWidth > window.innerWidth) left = window.innerWidth - fab.offsetWidth;
      if (bottom < 0) bottom = 0;
      if (bottom + fab.offsetHeight > window.innerHeight) bottom = window.innerHeight - fab.offsetHeight;
      fab.style.left = left + 'px';
      fab.style.bottom = bottom + 'px';
      fab.style.right = 'auto';
    });

    document.addEventListener('mouseup', function() {
      if (!isDragging) return;
      isDragging = false;
      fab.classList.remove('dragging');
      document.body.style.userSelect = '';
      savePosition(fab.offsetLeft, (window.innerHeight - (fab.offsetTop + fab.offsetHeight)));
    });

    // 点击打开/关闭面板，拖拽不触发点击
    var lastClickTime = 0;
    fab.onclick = function () {
      if (Date.now() - lastClickTime > 50) {
        panel.classList.toggle('open');
      }
    };
    // 记录点击时间，避免拖拽误触发点击
    fab.addEventListener('mousedown', function() {
      lastClickTime = Date.now();
    });

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
    function appendStep(step) {
      // 执行过程气泡
      var el = document.createElement('div');
      el.className = 'msg step';
      el.style.cssText = 'background:#f0f9ff;border:1px solid #bae7ff;color:#0050b3;font-size:13px;padding:8px 10px;border-radius:12px;max-width:90%;margin:8px 0;';
      el.textContent = step.text || step;
      msgs.appendChild(el);
      msgs.scrollTop = msgs.scrollHeight;
    }
    append('assistant', options.welcome || '你好，我是 EASP AI 助手。');
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
      console.log('[EASPAssistant] got token from tokenProvider:', token ? `${token.substring(0, 20)}...` : 'empty');
      // Normalize baseUrl: remove trailing slash
      var normalizedBaseUrl = baseUrl.replace(/\/$/, '');
      var payload = {
        session_id: conversationId || undefined,
        message: text,
        assistant_name: options.title || 'EASP AI 助手',
        execution_mode: executionMode,
        page_context: (options.pageContextProvider && options.pageContextProvider()) || { url: location.href, title: document.title }
      };
      console.log('[EASPAssistant] sending chat request to:', normalizedBaseUrl + '/api/embed/v1/assistant/chat', payload);
      var res = await fetch(normalizedBaseUrl + '/api/embed/v1/assistant/chat', { method: 'POST', headers: { 'Content-Type': 'application/json', 'easp-api-token': token }, body: JSON.stringify(payload) });
        if (res.status === 401) { console.log('[EASPAssistant] 401, refreshing token'); token = await options.tokenProvider(); res = await fetch(normalizedBaseUrl + '/api/embed/v1/assistant/chat', { method: 'POST', headers: { 'Content-Type': 'application/json', 'easp-api-token': token }, body: JSON.stringify(payload) }); }
        if (!res.ok) { var errTxt = await res.text(); console.error('[EASPAssistant] request failed:', res.status, errTxt); throw new Error(errTxt); }
        console.log('[EASPAssistant] request started, reading SSE');
        await readSSE(res, function (chunk) { assistantText += chunk; append('assistant', assistantText); }, appendStep, function (id) { conversationId = id; localStorage.setItem('easp_embed_conversation_id', id); console.log('[EASPAssistant] new conversation:', id); });
      } catch (err) {
        console.error('[EASPAssistant] error:', err);
        append('assistant', '请求失败：' + (err && err.message ? err.message : String(err)));
      }
    }
    send.onclick = submit;
    input.addEventListener('keydown', function (e) { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); } });
    return { open: function () { panel.classList.add('open'); }, close: function () { panel.classList.remove('open'); }, destroy: function () { host.remove(); } };
  }
  window.EASPAssistant = { init: init };
})();