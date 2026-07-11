/**
 * Agent SDK 类型定义
 *
 * [INPUT]: 无
 * [OUTPUT]: 对外提供 SessionId, ToolInput
 * [POS]: types 模块的 SDK 基础类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

/** SDK Session ID — Agent SDK 生成的 session 标识 */
export type SessionId = string;

/** 工具参数在具体工具消费前保持未知，禁止协议层替调用方猜测字段。 */
export type ToolInput = Record<string, unknown>;
