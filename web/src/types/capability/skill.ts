/**
 * Skill Marketplace 类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供当前 API 实际消费的 Skill 列表、详情、导入、Agent 启停和更新类型；不保留旧批量安装/更新响应
 * [POS]: types 模块的 Skill Marketplace 核心类型，被 skill-api.ts 和 skills 页面消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

export type SkillSourceType = "system" | "builtin" | "external" | "workspace";

export interface SkillInfo {
    name: string;
    title: string;
    description: string;
    scope: "main" | "any" | "room";
    tags: string[];
    category_key: string;
    category_name: string;
    source_type: SkillSourceType;
    source_ref: string;
    version: string;
    enabled_for_agent: boolean;
    locked: boolean;
    has_update: boolean;
    deletable: boolean;
    source_kind?: string;
    source_name?: string;
    source_trust?: string;
    import_mode?: string;
    last_error?: string;
    storage_scope?: "platform" | "user_global" | "agent_workspace" | string;
    origin_kind?: "builtin" | "user_import" | "marketplace" | "agent_created" | string;
    enabled_agent_count?: number;
}

export interface SkillDetail extends SkillInfo {
    readme_markdown: string;
    recommendation: string;
    deploy_successes?: RedeployAgentSuccess[];
    deploy_failures?: RedeployAgentFailure[];
}

export type AgentSkillEntry = SkillInfo;

export interface SkillAgentBinding {
    agent_id: string;
    agent_name: string;
    is_main: boolean;
    available: boolean;
    enabled: boolean;
}

export type SkillBindingTargetScope = "global_library" | "agent_workspace";

export interface SkillActionFailure {
    skill_name: string;
    error: string;
}

export interface RedeployAgentSuccess {
    agent_id: string;
    agent_name: string;
}

export interface RedeployAgentFailure extends RedeployAgentSuccess {
    error: string;
}

export interface CheckSkillUpdatesResponse {
    available_skills: string[];
    skipped_skills: string[];
    failures: SkillActionFailure[];
}

export interface ExternalSkillSearchItem {
    name: string;
    title: string;
    description: string;
    source: string;
    package_spec: string;
    skill_slug: string;
    installs: number;
    detail_url: string;
    readme_markdown: string;
    source_kind: string;
    source_key: string;
    source_name: string;
    source_trust: string;
    import_mode: string;
    git_url: string;
    git_branch: string;
    git_path: string;
    raw_url: string;
    tags: string[];
    version: string;
    artifact_sha256?: string;
    artifact_size?: number;
}

export interface ExternalSkillSourceStatus {
    key: string;
    name: string;
    kind: string;
    url: string;
    status: string;
    error?: string;
}

export interface ExternalSkillSourceInfo {
    source_id: string;
    name: string;
    kind: string;
    url: string;
    trust: string;
    enabled: boolean;
    sort_order: number;
    last_checked_at?: string;
    last_error?: string;
    managed_by: "system" | "user" | string;
    auth_type: "none" | "bearer" | string;
    credential_configured: boolean;
    deletable: boolean;
}

export interface ExternalSkillSourceRequest {
    name?: string;
    enabled?: boolean;
    auth_type?: "none" | "bearer";
    token?: string;
}

export interface CreateExternalSkillSourceRequest {
    name: string;
    url: string;
    auth_type: "none" | "bearer";
    token?: string;
}

export interface SearchExternalSkillsResponse {
    query: string;
    results: ExternalSkillSearchItem[];
    sources: ExternalSkillSourceStatus[];
}

export interface ExternalSkillPreviewResponse {
    detail_url: string;
    readme_markdown: string;
}
