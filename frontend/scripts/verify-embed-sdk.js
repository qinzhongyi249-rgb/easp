#!/usr/bin/env node
// 构建后验证：防止独立 JS SDK 特性丢失/被覆盖
// - 校验 public/embed/assistant.js 与 dist/embed/assistant.js MD5 一致（Vite 自动复制未失效）
// - 校验关键特性字符串都在（防止未来某次误改把功能移走）

import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const ROOT = path.resolve(__dirname, '..');
const PUBLIC_JS = path.join(ROOT, 'public/embed/assistant.js');
const DIST_JS = path.join(ROOT, 'dist/embed/assistant.js');

function md5(p) {
  return crypto.createHash('md5').update(fs.readFileSync(p)).digest('hex');
}

function check(name, cond) {
  if (cond) {
    console.log(`  ✓ ${name}`);
    return true;
  }
  console.error(`  ✗ ${name}`);
  return false;
}

console.log('[verify-embed-sdk] 校验独立 JS SDK 构建产物');

if (!fs.existsSync(PUBLIC_JS)) {
  console.error(`[verify-embed-sdk] MISSING: ${PUBLIC_JS}`);
  process.exit(1);
}
if (!fs.existsSync(DIST_JS)) {
  console.error(`[verify-embed-sdk] MISSING: ${DIST_JS}（Vite 未复制？）`);
  process.exit(1);
}

const publicHash = md5(PUBLIC_JS);
const distHash = md5(DIST_JS);
const publicSrc = fs.readFileSync(PUBLIC_JS, 'utf8');
const distSrc = fs.readFileSync(DIST_JS, 'utf8');

const results = [];

// 1. public 与 dist 完全一致
results.push(check(`public/dist md5 一致 (${publicHash})`, publicHash === distHash));

// 2. 关键特性必须都在 dist 产物里
const requiredFeatures = [
  { name: '正确的后端请求路径 /api/embed/v1/assistant/chat', needle: '/api/embed/v1/assistant/chat' },
  { name: '智能面板定位 updatePanelPosition', needle: 'updatePanelPosition' },
  { name: 'Markdown 块级解析器 markdownToHtml', needle: 'markdownToHtml' },
  { name: '表格宫格 wrap .easp-table-wrap', needle: 'easp-table-wrap' },
  { name: 'XSS 防护 escapeHtml', needle: 'escapeHtml' },
  { name: '步骤气泡 step-panel', needle: 'step-panel' },
  { name: 'z-index 分层（面板低于浮球）', needle: 'z-index:2147483646' },
  // 契约字段
  { name: 'request body 使用 message（单字符串）', needle: 'message: question' },
  { name: 'session_id 存储键 easp_embed_session_id', needle: 'easp_embed_session_id' },
];
for (const feat of requiredFeatures) {
  results.push(check(feat.name, distSrc.includes(feat.needle)));
}

// 3. 反向断言：不允许出现已知的错误路径 / 错误契约字段
const badPatterns = [
  { name: '不能残留旧路径 /embed/v1/assistant/chat（缺 /api 前缀）',
    reg: /["']\/embed\/v1\/assistant\/chat["']/ },
  { name: '不能发送 messages 数组（后端要求 message 单字符串）',
    reg: /messages:\s*\[\s*\{\s*role:\s*["']user["']/ },
  { name: '不能发送 tenant_id/user 字段（后端 EmbedChatRequest 无此字段）',
    reg: /body\.(tenant_id|user)\s*=/ },
];
for (const bp of badPatterns) {
  results.push(check(bp.name, !bp.reg.test(distSrc)));
}

const failed = results.filter((r) => !r).length;
if (failed > 0) {
  console.error(`\n[verify-embed-sdk] ${failed} 项失败，构建被视为不完整。`);
  console.error('  → 请检查 frontend/public/embed/assistant.js 是否被误改；');
  console.error('  → 或 Vite public/ 复制机制是否失效（vite.config.ts）。');
  process.exit(1);
}
console.log(`\n[verify-embed-sdk] 全部 ${results.length} 项通过`);
