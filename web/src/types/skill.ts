/**
 * Skill 类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 SkillInfo / AgentSkillEntry
 * [POS]: types 模块的 Skill 核心类型，被 skill-api.ts 和 contacts-profile-panel 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

/** Skill 元信息 — 从后端 registry 获取 */
export interface SkillInfo {
    name: string;
    description: string;
    scope: 'main' | 'any';
    tags: string[];
}

/** Agent 已安装 Skill 条目 */
export interface AgentSkillEntry {
    name: string;
    description: string;
    scope: string;
    tags: string[];
    installed: boolean;
    locked: boolean;
}
