/**
 * API 通用类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 ApiResponse
 * [POS]: types 模块的 API 基础类型，被 agent-api.ts 和 agent-manage-api.ts 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

// ==================== API 响应 ====================

/** 后端统一响应格式 */
export interface ApiResponse<T> {
    code: number;
    message: string;
    data: T;
    request_id?: string;
}
