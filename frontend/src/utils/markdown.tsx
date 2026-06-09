import React from 'react';

/**
 * 流式安全的 Markdown → HTML 渲染器
 * 关键：不依赖闭合标记（如 ``` 闭合），保证流式增量更新时也能正确渲染
 */
export function renderMarkdown(text: string): string {
  if (!text) return '';

  let html = text;

  // 1. 代码块：支持未闭合的 ```（流式场景）
  // 匹配 ```lang\n...``` 或 ```lang\n...（未闭合）
  html = html.replace(/```(\w*)\n([\s\S]*?)(?:```|$)/g, (_match, lang, code) => {
    const cls = lang ? ` class="language-${lang}"` : '';
    return `<pre><code${cls}>${escapeHtml(code)}</code></pre>`;
  });

  // 行内代码（排除已被代码块处理的部分）
  // 简化处理：只匹配行内代码
  html = html.replace(/`([^`\n]+)`/g, '<code>$1</code>');

  // 2. 标题（### 必须在 ## 前面）
  html = html.replace(/^#### (.*$)/gm, '<h4>$1</h4>');
  html = html.replace(/^### (.*$)/gm, '<h3>$1</h3>');
  html = html.replace(/^## (.*$)/gm, '<h2>$1</h2>');
  html = html.replace(/^# (.*$)/gm, '<h1>$1</h1>');

  // 3. 水平线
  html = html.replace(/^---+$/gm, '<hr/>');

  // 4. 粗体、斜体
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');

  // 5. 链接
  html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');

  // 6. 引用块
  html = html.replace(/^> (.*$)/gm, '<blockquote>$1</blockquote>');

  // 7. 表格（按行处理）
  html = html.replace(/((?:^\|.+\|$\n?)+)/gm, (tableBlock) => {
    const rows = tableBlock.trim().split('\n').filter(r => r.trim());
    if (rows.length === 0) return tableBlock;

    let result = '<table>';
    rows.forEach((row, idx) => {
      // 跳过分隔行 |---|---|
      if (/^\|[\s\-:|]+\|$/.test(row.trim())) return;
      const cells = row.split('|').filter(c => c.trim() !== '');
      if (cells.length === 0) return;
      const tag = idx === 0 ? 'th' : 'td';
      result += '<tr>' + cells.map(c => `<${tag}>${c.trim()}</${tag}>`).join('') + '</tr>';
    });
    result += '</table>';
    return result;
  });

  // 8. 列表项
  html = html.replace(/^\s*[-*] (.*$)/gm, '<li>$1</li>');
  html = html.replace(/^\s*\d+\. (.*$)/gm, '<li>$1</li>');
  // 连续 <li> 包裹进 <ul>
  html = html.replace(/((?:<li>.*<\/li>\n?)+)/g, '<ul>$1</ul>');

  // 9. 段落和换行
  html = html.replace(/\n\n/g, '</p><p>');
  html = html.replace(/\n/g, '<br/>');

  return `<div class="markdown-body"><p>${html}</p></div>`;
}

function escapeHtml(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/** Markdown 渲染组件 */
export const MarkdownRenderer: React.FC<{ content: string; compact?: boolean }> = ({ content, compact }) => {
  return (
    <div
      dangerouslySetInnerHTML={{ __html: renderMarkdown(content) }}
      style={compact ? { lineHeight: 1.7, fontSize: 13 } : { lineHeight: 1.8, fontSize: 14 }}
    />
  );
};

/** markdown-body 通用 CSS（注入到 style 标签） */
export const MARKDOWN_CSS = `
  .markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4 {
    margin-top: 12px; margin-bottom: 8px; font-weight: 600;
  }
  .markdown-body h1 { font-size: 18px; }
  .markdown-body h2 { font-size: 16px; }
  .markdown-body h3 { font-size: 15px; }
  .markdown-body h4 { font-size: 14px; }
  .markdown-body pre {
    background: #f5f5f5; padding: 12px; border-radius: 6px;
    overflow-x: auto; margin: 8px 0;
  }
  .markdown-body code {
    background: #f5f5f5; padding: 2px 6px; border-radius: 3px; font-size: 13px;
    font-family: 'SFMono-Regular', Consolas, monospace;
  }
  .markdown-body pre code { background: none; padding: 0; font-size: 12px; }
  .markdown-body ul, .markdown-body ol { padding-left: 20px; margin: 8px 0; }
  .markdown-body li { margin: 2px 0; }
  .markdown-body table { border-collapse: collapse; width: 100%; margin: 8px 0; }
  .markdown-body td, .markdown-body th {
    border: 1px solid #e8e8e8; padding: 8px 12px; text-align: left;
  }
  .markdown-body th { background: #fafafa; font-weight: 600; }
  .markdown-body a { color: #1677ff; text-decoration: none; }
  .markdown-body a:hover { text-decoration: underline; }
  .markdown-body p { margin: 4px 0; }
  .markdown-body strong { font-weight: 600; }
  .markdown-body blockquote {
    margin: 8px 0; padding: 4px 12px; border-left: 3px solid #d9d9d9;
    color: #666; background: #fafafa;
  }
  .markdown-body hr { border: none; border-top: 1px solid #e8e8e8; margin: 12px 0; }
`;
