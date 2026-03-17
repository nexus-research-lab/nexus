/**
 * 工具函数库
 *
 * [INPUT]: 依赖 clsx, tailwind-merge
 * [OUTPUT]: 对外提供 cn, formatRelativeTime, formatFileSize, formatTokens, formatCost, truncate
 * [POS]: lib 模块的通用工具层，被组件和其他 lib 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { type ClassValue, clsx } from 'clsx';
import { twMerge } from 'tailwind-merge';

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
export function formatRelativeTime(timestamp: number): string {
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
 * 格式化文件大小
 */
export function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';

  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));

  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
}

/**
 * 格式化Token数量
 */
export function formatTokens(tokens: number): string {
  if (tokens < 1000) return tokens.toString();
  if (tokens < 1000000) return `${(tokens / 1000).toFixed(1)}K`;
  return `${(tokens / 1000000).toFixed(1)}M`;
}

/**
 * 格式化成本
 */
export function formatCost(usd: number): string {
  if (usd < 0.01) return `$${usd.toFixed(4)}`;
  if (usd < 1) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(2)}`;
}

// ==================== 文本工具 ====================

/**
 * 截断文本
 */
export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
}
