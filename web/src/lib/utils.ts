/**
 * 工具函数库
 *
 * [INPUT]: 依赖 clsx, tailwind-merge
 * [OUTPUT]: 对外提供 cn, formatRelativeTime, formatTokens, formatCost, truncate
 * [POS]: lib 模块的通用工具层，被组件和其他 lib 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';
import { prepareWithSegments } from '@chenglou/pretext';

// ==================== 样式工具 ====================

/**
 * 合并Tailwind CSS类名
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

// ==================== 格式化工具 ====================

/**
 * 格式化相对时间
 */
export function format_relative_time(timestamp: number): string {
  if (!Number.isFinite(timestamp) || timestamp <= 0) {
    return '刚刚';
  }

  const normalizedTimestamp = timestamp < 1_000_000_000_000 ? timestamp * 1000 : timestamp;
  const now = Date.now();
  const diff = Math.max(0, now - normalizedTimestamp);

  const seconds = Math.floor(diff / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) return `${days}天前`;
  if (hours > 0) return `${hours}小时前`;
  if (minutes > 0) return `${minutes}分钟前`;
  if (seconds > 0) return `${seconds}秒前`;
  return '刚刚';
}

/**
 * 格式化Token数量
 */
export function format_tokens(tokens: number): string {
  if (tokens < 1000) return tokens.toString();
  if (tokens < 1000000) return `${(tokens / 1000).toFixed(1)}K`;
  return `${(tokens / 1000000).toFixed(1)}M`;
}

/**
 * 格式化成本
 */
export function format_cost(usd: number): string {
  if (usd < 0.01) return `$${usd.toFixed(4)}`;
  if (usd < 1) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(2)}`;
}

// ==================== 文本工具 ====================

/**
 * 截断文本（grapheme-aware，正确处理 CJK、emoji、ZWJ 序列）
 *
 * text.substring(0, N) 按 UTF-16 code unit 截，会把 emoji（surrogate pair）
 * 或 ZWJ 序列切成乱码。pretext prepareWithSegments 返回 grapheme cluster 数组，
 * 按 grapheme 数截断才是语言学上正确的做法。
 */
export function truncate(text: string, maxLength: number): string {
  // Fast path: ASCII-only，code unit 数 == grapheme 数，直接跳过 pretext
  if (text.length <= maxLength && !/[\uD800-\uDFFF\u0300-\u036F]/.test(text)) return text;
  try {
    const prepared = prepareWithSegments(text, '');
    const graphemes = prepared.segments;
    if (graphemes.length <= maxLength) return text;
    return graphemes.slice(0, maxLength).join('') + '…';
  } catch {
    // fallback to substring if pretext fails
    if (text.length <= maxLength) return text;
    return text.substring(0, maxLength) + '…';
  }
}

// ==================== 头像工具 ====================

/**
 * 获取名称缩写
 */
export function get_initials(
  name: string | null | undefined,
  fallback = 'AG',
  maxLength = 2,
): string {
  if (!name) {
    return fallback;
  }

  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) {
    return fallback;
  }

  if (parts.length === 1) {
    return parts[0].slice(0, maxLength).toUpperCase();
  }

  return parts
    .slice(0, maxLength)
    .map((part) => part[0] ?? '')
    .join('')
    .toUpperCase();
}

/**
 * 将头像标识解析为可直接使用的图片地址。
 *
 * 兼容三种输入：
 * 1. 纯数字 / 约定字符串：映射到本地 `/icon/*.png`
 * 2. 绝对 URL / data URL / blob URL
 * 3. 站内绝对路径
 */
export function get_icon_avatar_src(avatar: string | null | undefined): string | null {
  const normalizedAvatar = avatar?.trim();
  if (!normalizedAvatar) {
    return null;
  }

  if (
    normalizedAvatar.startsWith('http://')
    || normalizedAvatar.startsWith('https://')
    || normalizedAvatar.startsWith('data:')
    || normalizedAvatar.startsWith('blob:')
    || normalizedAvatar.startsWith('/')
  ) {
    return normalizedAvatar;
  }

  return `/icon/${normalizedAvatar}.png`;
}

/**
 * 根据字符串稳定生成区间内的图标编号。
 */
export function get_stable_icon_id(
  seed: string | null | undefined,
  startInclusive: number,
  endInclusive: number,
): string {
  const normalizedSeed = seed?.trim() || 'nexus';
  const range = endInclusive - startInclusive + 1;
  let hash = 0;

  for (let index = 0; index < normalizedSeed.length; index += 1) {
    hash = (hash * 31 + normalizedSeed.charCodeAt(index)) >>> 0;
  }

  return String(startInclusive + (hash % range));
}

/**
 * 房间头像默认使用 13-24 号像素图标，保证未设置时也有稳定的视觉锚点。
 */
export function get_room_avatar_icon_id(
  roomId: string | null | undefined,
  roomName: string | null | undefined,
  explicitAvatar?: string | null,
): string {
  return explicitAvatar?.trim() || get_stable_icon_id(roomId || roomName, 13, 24);
}
