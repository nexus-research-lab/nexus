/**
 * Skill Marketplace 类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 Skill 列表、详情、导入、安装和更新相关类型
 * [POS]: types 模块的 Skill Marketplace 核心类型，被 skill-api.ts 和 skills 页面消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

export type SkillSourceType = "system" | "builtin" | "external";

export interface SkillInfo {
    name: string;
    title: string;
    description: string;
    scope: "main" | "any";
    tags: string[];
    category_key: string;
    category_name: string;
    source_type: SkillSourceType;
    source_ref: string;
    version: string;
    installed: boolean;
    locked: boolean;
    has_update: boolean;
    deletable: boolean;
}

export interface SkillDetail extends SkillInfo {
    readme_markdown: string;
    recommendation: string;
}

export interface AgentSkillEntry extends SkillInfo {}

export interface SkillActionFailure {
    skill_name: string;
    error: string;
}

export interface BatchInstallSkillsResponse {
    successes: string[];
    failures: SkillActionFailure[];
}

export interface UpdateInstalledSkillsResponse {
    updated_skills: string[];
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
}

export interface SearchExternalSkillsResponse {
    query: string;
    results: ExternalSkillSearchItem[];
}

export interface ExternalSkillPreviewResponse {
    detail_url: string;
    readme_markdown: string;
}
