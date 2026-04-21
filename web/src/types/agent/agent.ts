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
export type AgentProvider = string;

export interface AgentOptions {
    provider?: AgentProvider;
    permission_mode?: string;
    allowed_tools?: string[];
    disallowed_tools?: string[];
    cwd?: string;
    max_turns?: number;
    max_thinking_tokens?: number;
    mcp_servers?: Record<string, any>;
    setting_sources?: ('user' | 'project')[];
}

// ==================== Agent 数据结构 ====================

/** 标准化的 Agent 数据结构 */
export interface Agent {
    agent_id: string;
    name: string;
    workspace_path: string;
    display_name?: string | null;
    headline?: string | null;
    profile_markdown?: string | null;
    options: AgentOptions;
    created_at: number;
    status: string;
    avatar?: string | null;
    description?: string | null;
    vibe_tags?: string[] | null;
    skills_count?: number | null;
}

export interface AgentRuntimeStatus {
    agent_id: string;
    running_task_count: number;
    status: 'running' | 'idle';
}

/** API 响应中的 Agent 数据（后端格式） */
export interface ApiAgent {
    agent_id: string;
    name: string;
    workspace_path: string;
    display_name?: string | null;
    headline?: string | null;
    profile_markdown?: string | null;
    options: Record<string, any> | null;
    created_at: string;
    status: string;
    avatar?: string | null;
    description?: string | null;
    vibe_tags?: string[] | null;
    skills_count?: number | null;
}

/** API 响应中的 Agent 会话数据（后端格式） */
export interface ApiAgentSession {
    session_key: string;
    agent_id: string;
    session_id: string | null;
    room_session_id?: string | null;
    room_id?: string | null;
    conversation_id?: string | null;
    channel_type: string;
    chat_type: string;
    status: string;
    created_at: string;
    last_activity: string;
    title: string | null;
    message_count: number;
    options: Record<string, any> | null;
}

/** 标准化的 Agent 会话数据结构 */
export interface AgentSession {
    session_key: string;
    agent_id: string;
    session_id: string | null;
    room_session_id: string | null;
    room_id: string | null;
    conversation_id: string | null;
    channel_type: string;
    chat_type: string;
    status: string;
    created_at: number;
    last_activity_at: number;
    title: string;
    message_count: number;
    options: Record<string, unknown>;
}

// ==================== 操作参数 ====================

/** 创建 Agent 参数 */
export interface CreateAgentParams {
    name: string;
    options?: Partial<AgentOptions>;
    avatar?: string;
    description?: string;
    vibe_tags?: string[];
}

/** 更新 Agent 参数 */
export interface UpdateAgentParams {
    name?: string;
    options?: Partial<AgentOptions>;
    avatar?: string;
    description?: string;
    vibe_tags?: string[];
}

export interface AgentIdentityDraft {
    avatar?: string;
    description?: string;
    vibe_tags?: string[];
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
