/**
 * useAgentSession Hook 类型定义
 *
 * [INPUT]: 依赖 @/types 的 Message, ToolCall
 * [OUTPUT]: 对外提供 UseAgentSessionOptions, UseAgentSessionReturn
 * [POS]: hooks/agent 模块的类型
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { Message, ToolCall } from '@/types';
import { PendingPermission, PermissionDecisionPayload } from '@/types/permission';

// ==================== Hook 选项 ====================

export interface UseAgentSessionOptions {
    wsUrl?: string;
    agentId?: string | null;
    onError?: (error: Error) => void;
}

// ==================== Hook 返回值 ====================

export interface UseAgentSessionReturn {
    messages: Message[];
    toolCalls: ToolCall[];
    /** 当前 session 路由键 */
    sessionKey: string | null;
    isLoading: boolean;
    error: string | null;
    sendMessage: (content: string) => Promise<void>;
    startSession: () => void;
    loadSession: (key: string) => void;
    clearSession: () => void;
    resetSession: () => void;
    loadHistoryMessages: (key: string) => Promise<void>;
    stopGeneration: () => void;
    deleteRound: (roundId: string) => Promise<void>;
    regenerate: (roundId: string) => Promise<void>;
    pendingPermission: PendingPermission | null;
    sendPermissionResponse: (payload: PermissionDecisionPayload) => void;
}
