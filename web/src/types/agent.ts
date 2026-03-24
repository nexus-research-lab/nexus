/**
 * Agent 类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 Agent / AgentOptions / ApiAgent / CreateAgentParams / UpdateAgentParams
 * [POS]: types 模块的 Agent 核心类型，被 agent-api.ts 和 agent store 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

// ==================== Agent 配置 ====================

/** Agent 级别配置选项（映射 ClaudeAgentOptions 的 Agent 层字段） */
export interface AgentOptions {
    model?: string;
    permission_mode?: string;
    allowed_tools?: string[];
    disallowed_tools?: string[];
    system_prompt?: string;
    cwd?: string;
    max_turns?: number;
    max_thinking_tokens?: number;
    mcp_servers?: Record<string, any>;
    skills_enabled?: boolean;
    setting_sources?: ('user' | 'project' | 'local')[];
}

// ==================== Agent 数据结构 ====================

/** 标准化的 Agent 数据结构 */
export interface Agent {
    agent_id: string;
    name: string;
    workspace_path: string;
    options: AgentOptions;
    created_at: number;
    status: string;
}

/** API 响应中的 Agent 数据（后端格式） */
export interface ApiAgent {
    agent_id: string;
    name: string;
    workspace_path: string;
    options: Record<string, any> | null;
    created_at: string;
    status: string;
}

// ==================== 操作参数 ====================

/** 创建 Agent 参数 */
export interface CreateAgentParams {
    name: string;
    options?: Partial<AgentOptions>;
}

/** 更新 Agent 参数 */
export interface UpdateAgentParams {
    name?: string;
    options?: Partial<AgentOptions>;
}

/** Agent 名称校验结果 */
export interface AgentNameValidationResult {
    name: string;
    normalized_name: string;
    is_valid: boolean;
    is_available: boolean;
    workspace_path?: string | null;
    reason?: string | null;
}

export interface WorkspaceFileEntry {
    path: string;
    name: string;
    is_dir: boolean;
    size?: number | null;
    modified_at: string;
    depth: number;
}

export interface WorkspaceFileContent {
    path: string;
    content: string;
}

export interface WorkspaceEntryMutationResponse {
    path: string;
}

export interface WorkspaceEntryRenameResponse {
    path: string;
    new_path: string;
}
