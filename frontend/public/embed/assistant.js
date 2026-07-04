const DEFAULT_WELCOME = "你好，我是 EASP AI 助手。";

function getStyles() {
  return `
    .easp-fab{position:fixed;width:56px;height:56px;border-radius:50%;border:0;background:linear-gradient(135deg,#4f7cff,#2b5cff);color:#fff;box-shadow:0 10px 24px rgba(43,92,255,.3);font-size:22px;z-index:2147483647;cursor:pointer;user-select:none}
    .easp-fab.dragging{cursor:grabbing}
    .easp-fab img,.easp-fab svg{width:100%;height:100%;border-radius:50%;object-fit:cover}
    .easp-panel{position:fixed;width:min(380px,calc(100vw - 24px));height:min(720px,calc(100vh - 24px));background:#fff;border:1px solid #e5e7eb;border-radius:18px;box-shadow:0 16px 48px rgba(15,23,42,.18);z-index:2147483646;display:none;flex-direction:column;overflow:hidden;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
    .easp-panel.open{display:flex}
    .rb{right:20px;bottom:88px}
    .lb{left:20px;bottom:88px}
    .fab-rb{right:24px;bottom:24px}
    .fab-lb{left:24px;bottom:24px}
    .easp-hd{height:52px;display:flex;align-items:center;justify-content:space-between;padding:0 14px;background:#fff;border-bottom:1px solid #eef2f7;color:#0f172a;font-weight:600}
    .easp-brand{display:flex;align-items:center;gap:8px;font-size:14px}
    .easp-brand-dot{width:8px;height:8px;border-radius:50%;background:#4f7cff}
    .easp-actions{display:flex;gap:8px;align-items:center}
    .easp-new,.easp-close{width:28px;height:28px;border:0;border-radius:8px;background:#f8fafc;color:#475569;font-size:16px;cursor:pointer}
    .easp-msgs{flex:1;overflow:auto;padding:14px;background:#f8fafc}
    .msg{max-width:88%;margin:8px 0;padding:10px 12px;border-radius:16px;line-height:1.65;font-size:14px;white-space:pre-wrap;word-break:break-word}
    .msg.user{margin-left:auto;background:linear-gradient(135deg,#5b7cff,#4f7cff);color:#fff;border-bottom-right-radius:6px}
    .msg.assistant{background:#fff;border:1px solid #e5e7eb;color:#111827;border-bottom-left-radius:6px}
    .assistant-turn{max-width:92%;margin:8px 0}
    .assistant-turn .msg.assistant{max-width:100%;margin:0}
    .step-panel{margin-bottom:8px;border:1px solid #e2e8f0;border-radius:14px;background:#fff;overflow:hidden}
    .step-panel.hidden{display:none}
    .step-toggle{display:flex;align-items:center;justify-content:space-between;width:100%;padding:10px 12px;border:0;background:transparent;cursor:pointer;text-align:left}
    .step-summary{display:flex;align-items:center;gap:8px;color:#334155;font-size:13px;font-weight:600}
    .step-badge{width:18px;height:18px;border-radius:50%;display:inline-flex;align-items:center;justify-content:center;font-size:11px;color:#fff;background:#94a3b8;flex:none}
    .step-badge.running{background:#f59e0b;animation:pulse 1.8s infinite}
    .step-badge.done{background:#10b981}
    .step-badge.error{background:#ef4444}
    .step-arrow{color:#94a3b8;font-size:12px;transition:transform .2s ease}
    .step-panel.collapsed .step-arrow{transform:rotate(-90deg)}
    .step-body{padding:0 12px 12px}
    .step-panel.collapsed .step-body{display:none}
    .step-item{position:relative;padding:0 0 12px 18px;color:#475569;font-size:13px}
    .step-item:last-child{padding-bottom:0}
    .step-item::before{content:"";position:absolute;left:4px;top:18px;bottom:-4px;width:1px;background:#e2e8f0}
    .step-item:last-child::before{display:none}
    .step-item-head{position:relative;display:flex;align-items:center;justify-content:space-between;gap:12px}
    .step-item-head::before{content:"";position:absolute;left:-18px;top:6px;width:8px;height:8px;border-radius:50%;background:#94a3b8}
    .step-item.running .step-item-head::before{background:#f59e0b}
    .step-item.done .step-item-head::before{background:#10b981}
    .step-item.error .step-item-head::before{background:#ef4444}
    .step-title{font-weight:500;color:#334155}
    .step-meta{font-size:12px;color:#94a3b8;white-space:nowrap}
    .step-result{margin-top:6px;color:#64748b;white-space:pre-wrap;word-break:break-word}
    .answer-empty{color:#94a3b8}
    /* Markdown 渲染 */
    .answer-content{max-width:100%;overflow:hidden;line-height:1.7}
    .answer-content h1,.answer-content h2,.answer-content h3,.answer-content h4{margin:10px 0 6px;color:#0f172a;font-weight:600}
    .answer-content h1{font-size:18px}
    .answer-content h2{font-size:16px}
    .answer-content h3{font-size:15px}
    .answer-content h4{font-size:14px}
    .answer-content p{margin:6px 0}
    .answer-content ul,.answer-content ol{margin:6px 0;padding-left:20px}
    .answer-content li{margin:2px 0}
    .answer-content code{background:#f1f5f9;padding:2px 5px;border-radius:4px;font-size:12px;font-family:"SF Mono",Consolas,Menlo,monospace}
    .answer-content pre{background:#0f172a;color:#e2e8f0;padding:10px 12px;border-radius:8px;overflow-x:auto;font-size:12px;margin:8px 0}
    .answer-content pre code{background:transparent;color:inherit;padding:0}
    .answer-content a{color:#4f7cff;text-decoration:underline}
    .answer-content blockquote{border-left:3px solid #cbd5e1;padding:2px 10px;margin:8px 0;color:#64748b;background:#f8fafc}
    .answer-content hr{border:0;border-top:1px solid #e2e8f0;margin:12px 0}
    /* 表格宫格 + 横向滚动 */
    .easp-table-wrap{max-width:100%;overflow-x:auto;overflow-y:hidden;margin:8px 0;padding-bottom:6px;cursor:grab;-webkit-overflow-scrolling:touch;overscroll-behavior-x:contain;touch-action:pan-x pan-y;border-radius:6px}
    .easp-table-wrap.dragging{cursor:grabbing;user-select:none}
    .easp-table-wrap::-webkit-scrollbar{height:8px}
    .easp-table-wrap::-webkit-scrollbar-thumb{background:#cbd5e1;border-radius:999px}
    .easp-table-wrap::-webkit-scrollbar-track{background:transparent}
    .easp-table{border-collapse:collapse;width:max-content;min-width:100%;margin:0;table-layout:auto;white-space:nowrap;font-size:13px}
    .easp-table td,.easp-table th{border:1px solid #e2e8f0;padding:7px 10px;vertical-align:top;white-space:nowrap;word-break:normal;background:#fff}
    .easp-table th{background:#f8fafc;font-weight:600;color:#334155}
    .easp-table tr:nth-child(even) td{background:#fafbfc}
    .easp-ft{display:flex;gap:8px;padding:10px;border-top:1px solid #e5e7eb;background:#fff}
    .easp-input{flex:1;resize:none;border:1px solid #d1d5db;border-radius:12px;padding:9px 10px;outline:none;font:inherit}
    .easp-input:focus{border-color:#4f7cff;box-shadow:0 0 0 3px rgba(79,124,255,.14)}
    .easp-send{border:0;border-radius:12px;background:#4f7cff;color:#fff;padding:0 16px;cursor:pointer}
    .easp-send:disabled{opacity:.55;cursor:not-allowed}
    @keyframes pulse{0%{opacity:1}50%{opacity:.45}100%{opacity:1}}
    @media(max-width:600px){
      .easp-panel{right:0!important;left:0!important;bottom:0!important;width:100vw;height:85vh!important;border-radius:18px 18px 0 0}
      .easp-fab{right:18px!important;left:auto!important}
    }
  `;
}

function formatDuration(ms) {
  const value = Number(ms);
  if (!Number.isFinite(value) || value <= 0) return "";
  if (value >= 1000) return (value / 1000).toFixed(1) + "s";
  return Math.round(value) + "ms";
}

// HTML 转义（防 XSS）
function escapeHtml(text) {
  return String(text)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

// Markdown 行内解析：**粗体** *斜体* `代码` [文本](url)
function renderInline(text) {
  // 先保护代码 token（避免内容被其它规则转义）
  const codeTokens = [];
  text = text.replace(/`([^`\n]+)`/g, function(_, code) {
    codeTokens.push("<code>" + escapeHtml(code) + "</code>");
    return "\u0000CODE" + (codeTokens.length - 1) + "\u0000";
  });
  // 剩余部分转义
  text = escapeHtml(text);
  // 链接 [text](url)
  text = text.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g,
    '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
  // 粗体 **x**
  text = text.replace(/\*\*([^*\n]+)\*\*/g, "<strong>$1</strong>");
  // 斜体 *x*
  text = text.replace(/(^|[^*])\*([^*\n]+)\*(?!\*)/g, "$1<em>$2</em>");
  // 恢复代码
  text = text.replace(/\u0000CODE(\d+)\u0000/g, function(_, i) {
    return codeTokens[Number(i)] || "";
  });
  return text;
}

// Markdown 块级解析：逐行扫描，识别表格/代码块/标题/列表/引用/段落
function markdownToHtml(source) {
  if (typeof source !== "string" || !source) return "";
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const out = [];
  let paragraph = [];

  function flushParagraph() {
    if (!paragraph.length) return;
    const merged = paragraph.join(" ").trim();
    if (merged) out.push("<p>" + renderInline(merged) + "</p>");
    paragraph = [];
  }

  function isTableRow(s) {
    return /^\s*\|.*\|\s*$/.test(s);
  }
  function isTableDelimiter(s) {
    return /^\s*\|?\s*:?-{2,}:?(\s*\|\s*:?-{2,}:?)+\s*\|?\s*$/.test(s);
  }
  function splitTableCells(s) {
    const trimmed = s.trim().replace(/^\|/, "").replace(/\|$/, "");
    return trimmed.split("|").map(function(c) { return c.trim(); });
  }

  let i = 0;
  while (i < lines.length) {
    const line = lines[i];

    // 代码块 ```lang ... ```
    const fenceMatch = /^```(\w*)\s*$/.exec(line);
    if (fenceMatch) {
      flushParagraph();
      const codeLines = [];
      i++;
      while (i < lines.length && !/^```\s*$/.test(lines[i])) {
        codeLines.push(lines[i]);
        i++;
      }
      i++; // 跳过结束 ```
      out.push("<pre><code>" + escapeHtml(codeLines.join("\n")) + "</code></pre>");
      continue;
    }

    // 表格：当前行看起来是表格行 + 下一行是分隔线
    if (isTableRow(line) && i + 1 < lines.length && isTableDelimiter(lines[i + 1])) {
      flushParagraph();
      const header = splitTableCells(line);
      i += 2; // 跳过表头行和分隔线
      const rows = [];
      while (i < lines.length && isTableRow(lines[i])) {
        rows.push(splitTableCells(lines[i]));
        i++;
      }
      let table = '<div class="easp-table-wrap"><table class="easp-table"><thead><tr>';
      for (const h of header) table += "<th>" + renderInline(h) + "</th>";
      table += "</tr></thead><tbody>";
      for (const row of rows) {
        table += "<tr>";
        for (let k = 0; k < header.length; k++) {
          table += "<td>" + renderInline(row[k] || "") + "</td>";
        }
        table += "</tr>";
      }
      table += "</tbody></table></div>";
      out.push(table);
      continue;
    }

    // 标题 # ## ###
    const headingMatch = /^(#{1,4})\s+(.*)$/.exec(line);
    if (headingMatch) {
      flushParagraph();
      const level = headingMatch[1].length;
      out.push("<h" + level + ">" + renderInline(headingMatch[2].trim()) + "</h" + level + ">");
      i++;
      continue;
    }

    // 引用 >
    if (/^>\s?/.test(line)) {
      flushParagraph();
      const quoteLines = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) {
        quoteLines.push(lines[i].replace(/^>\s?/, ""));
        i++;
      }
      out.push("<blockquote>" + renderInline(quoteLines.join(" ")) + "</blockquote>");
      continue;
    }

    // 分割线 ---
    if (/^\s*(?:-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
      flushParagraph();
      out.push("<hr />");
      i++;
      continue;
    }

    // 无序列表 - / *
    if (/^\s*[-*]\s+/.test(line)) {
      flushParagraph();
      const items = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*[-*]\s+/, ""));
        i++;
      }
      let list = "<ul>";
      for (const it of items) list += "<li>" + renderInline(it) + "</li>";
      list += "</ul>";
      out.push(list);
      continue;
    }

    // 有序列表 1.
    if (/^\s*\d+\.\s+/.test(line)) {
      flushParagraph();
      const items = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(lines[i].replace(/^\s*\d+\.\s+/, ""));
        i++;
      }
      let list = "<ol>";
      for (const it of items) list += "<li>" + renderInline(it) + "</li>";
      list += "</ol>";
      out.push(list);
      continue;
    }

    // 空行：段落分隔
    if (!line.trim()) {
      flushParagraph();
      i++;
      continue;
    }

    // 普通段落累积
    paragraph.push(line);
    i++;
  }
  flushParagraph();
  return out.join("\n");
}

// 为表格 wrapper 绑定鼠标拖拽 + shift+wheel + 触控横滑
function enhanceScrollableTables(root) {
  if (!root || !root.querySelectorAll) return;
  const wraps = root.querySelectorAll(".easp-table-wrap");
  wraps.forEach(function(wrap) {
    if (wrap.dataset.easpScrollReady === "1") return;
    wrap.dataset.easpScrollReady = "1";

    let isDown = false;
    let startX = 0;
    let scrollLeft = 0;

    wrap.addEventListener("mousedown", function(e) {
      isDown = true;
      wrap.classList.add("dragging");
      startX = e.pageX - wrap.offsetLeft;
      scrollLeft = wrap.scrollLeft;
    });
    wrap.addEventListener("mouseleave", function() {
      isDown = false;
      wrap.classList.remove("dragging");
    });
    wrap.addEventListener("mouseup", function() {
      isDown = false;
      wrap.classList.remove("dragging");
    });
    wrap.addEventListener("mousemove", function(e) {
      if (!isDown) return;
      e.preventDefault();
      const x = e.pageX - wrap.offsetLeft;
      wrap.scrollLeft = scrollLeft - (x - startX);
    });
    // Shift + 滚轮 = 横向滚动
    wrap.addEventListener("wheel", function(e) {
      if (e.shiftKey && e.deltaY !== 0) {
        e.preventDefault();
        wrap.scrollLeft += e.deltaY;
      }
    }, { passive: false });
  });
}

async function readSSE(response, onEvent) {
  const reader = response.body && response.body.getReader ? response.body.getReader() : null;
  if (!reader) return;

  const decoder = new TextDecoder();
  let buffer = "";

  const flushBlock = async function(block) {
    if (!block.trim()) return;
    const lines = block.split("\n");
    let eventName = "message";
    const dataLines = [];

    for (const line of lines) {
      if (line.startsWith("event:")) {
        eventName = line.slice(6).trim() || "message";
      } else if (line.startsWith("data:")) {
        dataLines.push(line.slice(5).trim());
      }
    }

    if (!dataLines.length) return;

    const rawData = dataLines.join("\n");
    let data = rawData;
    try {
      data = JSON.parse(rawData);
    } catch (error) {
      data = rawData;
    }

    await onEvent({
      event: eventName,
      data: data,
    });
  };

  for (;;) {
    const result = await reader.read();
    if (result.done) break;

    buffer += decoder.decode(result.value, { stream: true });
    buffer = buffer.replace(/\r\n/g, "\n");

    let delimiterIndex = buffer.indexOf("\n\n");
    while (delimiterIndex !== -1) {
      const block = buffer.slice(0, delimiterIndex);
      buffer = buffer.slice(delimiterIndex + 2);
      await flushBlock(block);
      delimiterIndex = buffer.indexOf("\n\n");
    }
  }

  const remain = decoder.decode();
  if (remain) buffer += remain.replace(/\r\n/g, "\n");
  if (buffer.trim()) await flushBlock(buffer);
}

function createMessage(messagesEl, role, text) {
  const message = document.createElement("div");
  message.className = "msg " + role;
  if (role === "assistant") {
    // 助手消息用 markdown 渲染
    const inner = document.createElement("div");
    inner.className = "answer-content";
    inner.innerHTML = markdownToHtml(text || "");
    message.appendChild(inner);
    enhanceScrollableTables(inner);
  } else {
    message.textContent = text || "";
  }
  messagesEl.appendChild(message);
  messagesEl.scrollTop = messagesEl.scrollHeight;
  return message;
}

function createAssistantTurn(messagesEl) {
  const turnEl = document.createElement("div");
  turnEl.className = "assistant-turn";

  const stepPanel = document.createElement("section");
  stepPanel.className = "step-panel hidden";

  const toggleButton = document.createElement("button");
  toggleButton.className = "step-toggle";
  toggleButton.type = "button";

  const summary = document.createElement("span");
  summary.className = "step-summary";

  const badge = document.createElement("span");
  badge.className = "step-badge running";
  badge.textContent = "·";

  const summaryText = document.createElement("span");
  summaryText.textContent = "处理中";

  const arrow = document.createElement("span");
  arrow.className = "step-arrow";
  arrow.textContent = "▾";

  summary.appendChild(badge);
  summary.appendChild(summaryText);
  toggleButton.appendChild(summary);
  toggleButton.appendChild(arrow);

  const stepBody = document.createElement("div");
  stepBody.className = "step-body";

  stepPanel.appendChild(toggleButton);
  stepPanel.appendChild(stepBody);

  const answerEl = document.createElement("div");
  answerEl.className = "msg assistant";

  const answerText = document.createElement("div");
  answerText.className = "answer-empty";
  answerText.textContent = "正在生成回答...";
  answerEl.appendChild(answerText);

  turnEl.appendChild(stepPanel);
  turnEl.appendChild(answerEl);
  messagesEl.appendChild(turnEl);
  messagesEl.scrollTop = messagesEl.scrollHeight;

  let collapsed = false;
  let finished = false;
  let answer = "";
  const steps = [];

  toggleButton.addEventListener("click", function() {
    collapsed = !collapsed;
    stepPanel.classList.toggle("collapsed", collapsed);
  });

  function syncSummary(state, text, durationText) {
    badge.className = "step-badge " + state;
    badge.textContent = state === "done" ? "✓" : state === "error" ? "!" : "·";
    summaryText.textContent = durationText ? text + " · " + durationText : text;
  }

  function renderSteps() {
    stepBody.innerHTML = "";
    for (const step of steps) {
      const item = document.createElement("div");
      item.className = "step-item " + step.status;

      const head = document.createElement("div");
      head.className = "step-item-head";

      const title = document.createElement("span");
      title.className = "step-title";
      title.textContent = step.title;

      const meta = document.createElement("span");
      meta.className = "step-meta";
      meta.textContent = step.durationText || "";

      head.appendChild(title);
      head.appendChild(meta);
      item.appendChild(head);

      if (step.result) {
        const result = document.createElement("div");
        result.className = "step-result";
        result.textContent = step.result;
        item.appendChild(result);
      }

      stepBody.appendChild(item);
    }
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  return {
    appendDelta(delta) {
      if (typeof delta !== "string" || !delta) return;
      answer += delta;
      answerText.className = "answer-content";
      answerText.innerHTML = markdownToHtml(answer);
      enhanceScrollableTables(answerText);
      messagesEl.scrollTop = messagesEl.scrollHeight;
    },
    addStep(payload) {
      const title = payload && payload.title ? String(payload.title) : "";
      if (!title) return;

      stepPanel.classList.remove("hidden");

      const last = steps[steps.length - 1];
      if (last && last.status === "running") {
        last.status = "done";
      }

      let nextStatus = "running";
      if (payload.status === "error") nextStatus = "error";
      if (payload.status === "done") nextStatus = "done";

      steps.push({
        title: title,
        status: nextStatus,
        result: payload.result ? String(payload.result) : "",
        durationText: formatDuration(payload.elapsed_ms || payload.stage_ms),
      });

      syncSummary(nextStatus === "error" ? "error" : "running", "处理中", "");
      renderSteps();
    },
    finish(payload) {
      if (finished) return;
      finished = true;

      const last = steps[steps.length - 1];
      if (last && last.status === "running") {
        last.status = "done";
      }

      if (!answer) {
        answerText.className = "answer-content";
        const msg = payload && payload.message ? String(payload.message) : "";
        answerText.innerHTML = msg ? markdownToHtml(msg) : "";
      } else {
        // 最终态再渲染一次，防止流式过程中把未完成的 markdown 打断
        answerText.className = "answer-content";
        answerText.innerHTML = markdownToHtml(answer);
        enhanceScrollableTables(answerText);
      }

      if (steps.length) {
        const durationText = formatDuration(payload && (payload.total_ms || payload.stream_ms));
        syncSummary("done", "处理完成", durationText);
        renderSteps();
      }
      messagesEl.scrollTop = messagesEl.scrollHeight;
    },
    fail(message) {
      const last = steps[steps.length - 1];
      if (last && last.status === "running") {
        last.status = "error";
      }

      stepPanel.classList.remove("hidden");
      syncSummary("error", "处理失败", "");
      renderSteps();

      answerText.className = "";
      answerText.textContent = "请求失败：" + message;
      messagesEl.scrollTop = messagesEl.scrollHeight;
    },
  };
}

function initAssistant(options) {
  if (!options || typeof options.tokenProvider !== "function") {
    throw new Error("EASPAssistant.init: tokenProvider is required");
  }

  const baseUrl = String(options.baseUrl || "").replace(/\/$/, "");
  const panelPosition = options.position === "left-bottom" ? "lb" : "rb";
  const fabPosition = options.position === "left-bottom" ? "fab-lb" : "fab-rb";

  const host = document.createElement("div");
  const shadowRoot = host.attachShadow({ mode: "open" });

  let fabContent = "AI";
  if (options.iconUrl) {
    fabContent = '<img src="' + options.iconUrl + '" alt="" />';
  } else if (options.iconSvg) {
    fabContent = options.iconSvg;
  }

  shadowRoot.innerHTML =
    '<style>' + getStyles() + '</style>' +
    '<button class="easp-fab ' + fabPosition + '" aria-label="' + (options.title || "EASP AI 助手") + '">' + fabContent + '</button>' +
    '<section class="easp-panel ' + panelPosition + '">' +
      '<header class="easp-hd">' +
        '<div class="easp-brand"><span class="easp-brand-dot"></span><span>' + (options.title || "EASP AI 助手") + '</span></div>' +
        '<div class="easp-actions">' +
          '<button class="easp-new" title="新建对话">+</button>' +
          '<button class="easp-close" title="关闭">×</button>' +
        '</div>' +
      '</header>' +
      '<main class="easp-msgs"></main>' +
      '<footer class="easp-ft">' +
        '<textarea class="easp-input" rows="2" placeholder="输入你的需求..."></textarea>' +
        '<button class="easp-send">发送</button>' +
      '</footer>' +
    '</section>';

  document.body.appendChild(host);

  const fab = shadowRoot.querySelector(".easp-fab");
  const panel = shadowRoot.querySelector(".easp-panel");
  const closeButton = shadowRoot.querySelector(".easp-close");
  const newButton = shadowRoot.querySelector(".easp-new");
  const messagesEl = shadowRoot.querySelector(".easp-msgs");
  const inputEl = shadowRoot.querySelector(".easp-input");
  const sendButton = shadowRoot.querySelector(".easp-send");

  let conversationId = localStorage.getItem("easp_embed_conversation_id") || "";
  let apiToken = "";
  let dragging = false;
  let moved = false;
  let dragStartX = 0;
  let dragStartY = 0;
  let originLeft = 0;
  let originTop = 0;

  function scrollBottom() {
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  function addWelcomeMessage() {
    createMessage(messagesEl, "assistant", options.welcome || DEFAULT_WELCOME);
  }

  function resetConversation() {
    conversationId = "";
    localStorage.removeItem("easp_embed_conversation_id");
    messagesEl.innerHTML = "";
    addWelcomeMessage();
  }

  async function sendMessage() {
    const question = inputEl.value.trim();
    if (!question) return;

    inputEl.value = "";
    createMessage(messagesEl, "user", question);
    const turn = createAssistantTurn(messagesEl);

    sendButton.disabled = true;

    try {
      const body = {
        conversation_id: conversationId || undefined,
        messages: [{ role: "user", content: question }],
        page_context: typeof options.pageContextProvider === "function"
          ? options.pageContextProvider()
          : { url: location.href, title: document.title },
        execution_mode: options.execution_mode || options.executionMode || "normal",
      };

      if (options.user) body.user = options.user;
      if (options.tenantId) body.tenant_id = options.tenantId;

      let response = null;
      for (let attempt = 0; attempt < 2; attempt += 1) {
        apiToken = apiToken || await options.tokenProvider();
        response = await fetch(baseUrl + "/api/embed/v1/assistant/chat", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "easp-api-token": apiToken,
          },
          body: JSON.stringify(body),
        });

        if (response.status !== 401 || attempt === 1) {
          break;
        }

        apiToken = await options.tokenProvider();
      }

      if (!response.ok) {
        throw new Error(await response.text());
      }

      await readSSE(response, async function(packet) {
        const eventName = packet.event;
        const data = packet.data && typeof packet.data === "object" ? packet.data : {};

        if (data.conversation_id) {
          conversationId = data.conversation_id;
          localStorage.setItem("easp_embed_conversation_id", conversationId);
        }

        if (eventName === "status" || eventName === "step") {
          turn.addStep({
            title: data.title || data.message || data.step || eventName,
            status: data.status,
            result: data.result,
            elapsed_ms: data.elapsed_ms,
            stage_ms: data.stage_ms,
          });
          return;
        }

        if (eventName === "done") {
          turn.finish(data);
          return;
        }

        const delta = typeof data.content === "string" ? data.content : typeof data.delta === "string" ? data.delta : "";
        if (delta) {
          turn.appendDelta(delta);
        }
      });

      turn.finish({});
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      turn.fail(message);
    } finally {
      sendButton.disabled = false;
      scrollBottom();
    }
  }

  addWelcomeMessage();

  if (panelPosition === "rb") {
    fab.style.right = "24px";
    fab.style.bottom = "24px";
    fab.style.left = "auto";
    fab.style.top = "auto";

    panel.style.right = "20px";
    panel.style.bottom = "88px";
    panel.style.left = "auto";
    panel.style.top = "auto";
  } else {
    fab.style.left = "24px";
    fab.style.bottom = "24px";
    fab.style.right = "auto";
    fab.style.top = "auto";

    panel.style.left = "20px";
    panel.style.bottom = "88px";
    panel.style.right = "auto";
    panel.style.top = "auto";
  }

  function handleMouseDown(event) {
    dragging = true;
    moved = false;
    dragStartX = event.clientX;
    dragStartY = event.clientY;
    const rect = fab.getBoundingClientRect();
    originLeft = rect.left;
    originTop = rect.top;
    fab.classList.add("dragging");
    event.preventDefault();
  }

  function handleMouseMove(event) {
    if (!dragging) return;

    const deltaX = event.clientX - dragStartX;
    const deltaY = event.clientY - dragStartY;
    if (Math.abs(deltaX) > 3 || Math.abs(deltaY) > 3) {
      moved = true;
    }

    let nextLeft = originLeft + deltaX;
    let nextTop = originTop + deltaY;

    nextLeft = Math.max(0, Math.min(nextLeft, window.innerWidth - fab.offsetWidth));
    nextTop = Math.max(0, Math.min(nextTop, window.innerHeight - fab.offsetHeight));

    fab.style.left = nextLeft + "px";
    fab.style.top = nextTop + "px";
    fab.style.right = "auto";
    fab.style.bottom = "auto";

    updatePanelPosition();
  }

  // 智能面板定位：优先浮球上方，空间不够放下方，最后 clamp 到视口内。
  // 修复：浮球拖到屏幕上方时，弹框不再飞出屏幕；弹框永远不会覆盖浮球本身（fab z-index 更高）。
  function updatePanelPosition() {
    const fabRect = fab.getBoundingClientRect();
    // 面板尺寸取 CSS 定义的实际约束值
    const panelWidth = Math.min(380, window.innerWidth - 24);
    const panelHeight = Math.min(720, window.innerHeight - 24);
    const gap = 12; // 面板与浮球之间的间距
    const margin = 8; // 面板离视口边缘的最小距离

    // 优先放在浮球上方（bottom 定位），空间不足则放下方（top 定位）
    let panelTop;
    const spaceAbove = fabRect.top - gap - margin;
    const spaceBelow = window.innerHeight - fabRect.bottom - gap - margin;
    if (spaceAbove >= panelHeight || spaceAbove >= spaceBelow) {
      // 放上方
      panelTop = fabRect.top - gap - panelHeight;
      if (panelTop < margin) panelTop = margin;
    } else {
      // 放下方
      panelTop = fabRect.bottom + gap;
      if (panelTop + panelHeight > window.innerHeight - margin) {
        panelTop = window.innerHeight - margin - panelHeight;
      }
    }

    // 水平：默认与浮球左对齐，超出右边则贴右；小于 margin 则贴左
    let panelLeft = fabRect.left;
    if (panelLeft + panelWidth > window.innerWidth - margin) {
      panelLeft = window.innerWidth - margin - panelWidth;
    }
    if (panelLeft < margin) panelLeft = margin;

    panel.style.left = panelLeft + "px";
    panel.style.top = panelTop + "px";
    panel.style.right = "auto";
    panel.style.bottom = "auto";
  }

  function handleMouseUp() {
    dragging = false;
    fab.classList.remove("dragging");
  }

  fab.addEventListener("mousedown", handleMouseDown);
  document.addEventListener("mousemove", handleMouseMove);
  document.addEventListener("mouseup", handleMouseUp);

  fab.addEventListener("click", function() {
    if (moved) {
      moved = false;
      return;
    }
    const willOpen = !panel.classList.contains("open");
    if (willOpen) {
      // 用户拖动浮球后，打开面板前重算位置，防止弹框飞出屏幕/覆盖浮球
      updatePanelPosition();
    }
    panel.classList.toggle("open");
  });

  // 视口大小变化时（包括手机键盘弹出、旋屏）重算面板位置
  window.addEventListener("resize", function() {
    if (panel.classList.contains("open")) {
      updatePanelPosition();
    }
  });

  closeButton.addEventListener("click", function() {
    panel.classList.remove("open");
  });

  newButton.addEventListener("click", function() {
    resetConversation();
  });

  sendButton.addEventListener("click", function() {
    sendMessage();
  });

  inputEl.addEventListener("keydown", function(event) {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      sendMessage();
    }
  });

  return {
    open: function() {
      panel.classList.add("open");
    },
    close: function() {
      panel.classList.remove("open");
    },
    destroy: function() {
      fab.removeEventListener("mousedown", handleMouseDown);
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
      host.remove();
    },
  };
}

window.EASPAssistant = {
  init: initAssistant,
};
