/**
 * Skill API 服务模块
 *
 * [INPUT]: 依赖 @/types/skill, @/types/api
 * [OUTPUT]: 对外提供 getAvailableSkills、getAgentSkills、installSkill、uninstallSkill
 * [POS]: lib 模块的 Skill API 层，被 contacts-profile-panel 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { AgentSkillEntry, SkillInfo } from '@/types/skill';
import { ApiResponse } from '@/types/api';
import { getAgentApiBaseUrl } from '@/config/options';

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

/** 获取所有可用 Skill 清单 */
export const getAvailableSkillsApi = async (): Promise<SkillInfo[]> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/skills`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`获取 Skill 列表失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillInfo[]> = await response.json();
    return result.data;
};

/** 获取 Agent 的 Skill 列表（含安装状态） */
export const getAgentSkillsApi = async (agent_id: string): Promise<AgentSkillEntry[]> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/skills`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`获取 Agent Skills 失败: ${response.statusText}`);
    }
    const result: ApiResponse<AgentSkillEntry[]> = await response.json();
    return result.data;
};

/** 为 Agent 安装 Skill */
export const installSkillApi = async (agent_id: string, skill_name: string): Promise<AgentSkillEntry> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/skills`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ skill_name }),
    });
    if (!response.ok) {
        throw new Error(`安装 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<AgentSkillEntry> = await response.json();
    return result.data;
};

/** 从 Agent 卸载 Skill */
export const uninstallSkillApi = async (agent_id: string, skill_name: string): Promise<void> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/skills/${skill_name}`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`卸载 Skill 失败: ${response.statusText}`);
    }
};
