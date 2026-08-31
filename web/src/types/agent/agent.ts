/**
 * Agent 类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 Agent、owner-scoped 创建对账与带内容 revision 的 WorkspaceFileContent
 * [POS]: types 模块的 Agent 核心类型，被 agent-api.ts 和 agent store 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

// ==================== Agent 配置 ====================

/** Agent 级别配置选项（映射 SDK Agent 层字段） */
export type AgentProvider = string;

export interface AgentOptions {
    provider?: AgentProvider;
    model?: string;
    permission_mode?: string;
    allowed_tools?: string[];
    disallowed_tools?: string[];
    cwd?: string;
    max_turns?: number;
    max_thinking_tokens?: number;
    mcp_servers?: Record<string, unknown>;
    connector_ids?: string[];
    skill_ids?: string[];
    disabled_skill_ids?: string[];
    setting_sources?: ('user' | 'project')[];
}

// ==================== Agent 数据结构 ====================

/** 标准化的 Agent 数据结构 */
export interface Agent {
    agent_id: string;
    name: string;
    workspace_path: string;
    is_main?: boolean;
    display_name?: string | null;
    headline?: string | null;
    profile_markdown?: string | null;
    options: AgentOptions;
    created_at: number;
    status: string;
    avatar?: string | null;
    description?: string | null;
    business_tags?: string[] | null;
    vibe_tags?: string[] | null;
    skills_count?: number | null;
    /** 仅在 Room member 投影中存在；全局 Agent 本身不拥有此状态。 */
    room_participation_paused?: boolean;
}

/** 普通 Agent 通讯录中的双向好友投影。 */
export interface AgentContact {
    id: string;
    owner_agent_id: string;
    contact_agent_id: string;
    alias?: string;
    direct_room_id?: string;
    name: string;
    display_name?: string;
    avatar?: string;
    created_at: string;
    updated_at: string;
}

export interface AgentCommunicationSendResult {
    message_id: string;
    status: string;
    target_type: "agent" | "room";
    target_id: string;
    room_id: string;
    conversation_id: string;
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
    is_main?: boolean;
    display_name?: string | null;
    headline?: string | null;
    profile_markdown?: string | null;
    options: AgentOptions | null;
    created_at: string;
    status: string;
    avatar?: string | null;
    description?: string | null;
    business_tags?: string[] | null;
    vibe_tags?: string[] | null;
    skills_count?: number | null;
}

/** API 响应中的 Agent 会话数据（后端格式） */
export interface ExternalSessionIdentity {
    channel_type: string;
    account_hint?: string;
    legacy_session_hint?: string;
    account_status?: string;
    peer_name?: string;
    pairing_status: string;
    current_pairing: boolean;
    can_delete: boolean;
    task_reference_count?: number;
}

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
    options: Record<string, unknown> | null;
    external_identity?: ExternalSessionIdentity | null;
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
    external_identity?: ExternalSessionIdentity | null;
}

// ==================== 操作参数 ====================

/** 创建 Agent 参数 */
export interface CreateAgentParams {
    name: string;
    options?: Partial<AgentOptions>;
    avatar?: string;
    description?: string;
    profile_template?: string;
    business_tags?: string[];
    vibe_tags?: string[];
    creation_request_id?: string;
}

export type AgentCreationRequestStatus =
    | "not_found"
    | "pending"
    | "committed"
    | "deleted"
    | "failed";

export interface ApiAgentCreationRequestResult {
    creation_request_id: string;
    status: AgentCreationRequestStatus;
    agent?: ApiAgent | null;
}

export interface AgentCreationRequestResult {
    creationRequestId: string;
    status: AgentCreationRequestStatus;
    agent: Agent | null;
}

/** 更新 Agent 参数 */
export interface UpdateAgentParams {
    name?: string;
    options?: Partial<AgentOptions>;
    avatar?: string;
    description?: string;
    business_tags?: string[];
    vibe_tags?: string[];
}

export interface AgentIdentityDraft {
    avatar?: string;
    description?: string;
    profile_template?: string;
    business_tags?: string[];
    vibe_tags?: string[];
}

export interface AgentProfileTemplateResponse {
    content: string;
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
    revision: string;
}

export interface WorkspaceEntryMutationResponse {
    path: string;
}

export interface WorkspaceEntryRenameResponse {
    path: string;
    new_path: string;
}
