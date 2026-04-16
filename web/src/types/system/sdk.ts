/**
 * Claude Agent SDK 类型定义
 *
 * [INPUT]: 无
 * [OUTPUT]: 对外提供 UUID, SessionId, ToolInput, ToolOutput
 * [POS]: types 模块的 SDK 基础类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

// ==================== 基础类型 ====================

export type UUID = string;

/** SDK Session ID — Claude SDK 生成的 session 标识 */
export type SessionId = string;

export type ToolInput = Record<string, any>;
export type ToolOutput = Record<string, any>;