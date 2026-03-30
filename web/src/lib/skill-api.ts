/**
 * Skill Marketplace API 服务模块
 *
 * [INPUT]: 依赖 @/types/api, @/types/skill
 * [OUTPUT]: 对外提供 Skill 查询、导入、安装、批量安装、更新 API 函数
 * [POS]: lib 模块的 Skill API 层，被 skills 页面消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { ApiResponse } from "@/types/api";
import {
    AgentSkillEntry,
    BatchInstallSkillsResponse,
    ExternalSkillSearchItem,
    SkillDetail,
    SkillInfo,
    UpdateInstalledSkillsResponse,
} from "@/types/skill";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

interface GetSkillsParams {
    agent_id?: string;
    category_key?: string;
    source_type?: string;
    q?: string;
}

function buildQuery(params: Record<string, string | undefined>): string {
    const query = new URLSearchParams();
    Object.entries(params).forEach(([key, value]) => {
        if (!value) return;
        query.set(key, value);
    });
    const text = query.toString();
    return text ? `?${text}` : "";
}

export const getAvailableSkillsApi = async (params: GetSkillsParams = {}): Promise<SkillInfo[]> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/skills${buildQuery({
            agent_id: params.agent_id,
            category_key: params.category_key,
            source_type: params.source_type,
            q: params.q,
        })}`,
        {
            method: "GET",
            headers: { "Content-Type": "application/json" },
        },
    );
    if (!response.ok) {
        throw new Error(`获取 Skill 列表失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillInfo[]> = await response.json();
    return result.data;
};

export const getSkillDetailApi = async (
    skill_name: string,
    agent_id?: string,
): Promise<SkillDetail> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/skills/${encodeURIComponent(skill_name)}${buildQuery({ agent_id })}`,
        {
            method: "GET",
            headers: { "Content-Type": "application/json" },
        },
    );
    if (!response.ok) {
        throw new Error(`获取 Skill 详情失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillDetail> = await response.json();
    return result.data;
};

export const getAgentSkillsApi = async (agent_id: string): Promise<AgentSkillEntry[]> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/skills`, {
        method: "GET",
        headers: { "Content-Type": "application/json" },
    });
    if (!response.ok) {
        throw new Error(`获取 Agent Skill 列表失败: ${response.statusText}`);
    }
    const result: ApiResponse<AgentSkillEntry[]> = await response.json();
    return result.data;
};

export const installSkillApi = async (agent_id: string, skill_name: string): Promise<AgentSkillEntry> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/skills`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ skill_name }),
    });
    if (!response.ok) {
        throw new Error(`安装 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<AgentSkillEntry> = await response.json();
    return result.data;
};

export const uninstallSkillApi = async (agent_id: string, skill_name: string): Promise<void> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/skills/${encodeURIComponent(skill_name)}`,
        {
            method: "DELETE",
            headers: { "Content-Type": "application/json" },
        },
    );
    if (!response.ok) {
        throw new Error(`卸载 Skill 失败: ${response.statusText}`);
    }
};

export const batchInstallSkillsApi = async (
    agent_id: string,
    skill_names: string[],
): Promise<BatchInstallSkillsResponse> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/skills/batch-install`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ skill_names }),
    });
    if (!response.ok) {
        throw new Error(`批量安装 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<BatchInstallSkillsResponse> = await response.json();
    return result.data;
};

export const updateInstalledSkillsApi = async (
    agent_id: string,
): Promise<UpdateInstalledSkillsResponse> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${encodeURIComponent(agent_id)}/skills/update-installed`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
    });
    if (!response.ok) {
        throw new Error(`更新已安装 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<UpdateInstalledSkillsResponse> = await response.json();
    return result.data;
};

export const updateImportedSkillsApi = async (): Promise<UpdateInstalledSkillsResponse> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/skills/update-imported`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
    });
    if (!response.ok) {
        throw new Error(`更新全局 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<UpdateInstalledSkillsResponse> = await response.json();
    return result.data;
};

export const updateSingleSkillApi = async (
    skill_name: string,
): Promise<SkillDetail> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/skills/${encodeURIComponent(skill_name)}/update`,
        {
            method: "POST",
            headers: { "Content-Type": "application/json" },
        },
    );
    if (!response.ok) {
        throw new Error(`更新 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillDetail> = await response.json();
    return result.data;
};

export const setSkillGlobalEnabledApi = async (
    skill_name: string,
    enabled: boolean,
): Promise<SkillDetail> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/skills/${encodeURIComponent(skill_name)}/global-enabled`,
        {
            method: "PATCH",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ enabled }),
        },
    );
    if (!response.ok) {
        throw new Error(`设置 Skill 全局启用状态失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillDetail> = await response.json();
    return result.data;
};

export const deleteSkillFromPoolApi = async (skill_name: string): Promise<void> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/skills/${encodeURIComponent(skill_name)}`,
        {
            method: "DELETE",
            headers: { "Content-Type": "application/json" },
        },
    );
    if (!response.ok) {
        throw new Error(`从技能池删除 Skill 失败: ${response.statusText}`);
    }
};

export const importGitSkillApi = async (url: string, branch?: string): Promise<SkillDetail> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/skills/import/git`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url, branch }),
    });
    if (!response.ok) {
        throw new Error(`Git 导入 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillDetail> = await response.json();
    return result.data;
};

export const importLocalSkillApi = async (file: File): Promise<SkillDetail> => {
    const form_data = new FormData();
    form_data.append("file", file);
    const response = await fetch(`${AGENT_API_BASE_URL}/skills/import/local`, {
        method: "POST",
        body: form_data,
    });
    if (!response.ok) {
        throw new Error(`本地导入 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillDetail> = await response.json();
    return result.data;
};

export const searchExternalSkillsApi = async (query: string): Promise<ExternalSkillSearchItem[]> => {
    const response = await fetch(
        `${AGENT_API_BASE_URL}/skills/search/external${buildQuery({ q: query })}`,
        {
            method: "GET",
            headers: { "Content-Type": "application/json" },
        },
    );
    if (!response.ok) {
        throw new Error(`搜索外部 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<{ query: string; results: ExternalSkillSearchItem[] }> = await response.json();
    return result.data.results;
};

export const importSkillsShSkillApi = async (
    package_spec: string,
    skill_slug: string,
): Promise<SkillDetail> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/skills/import/skills-sh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ package_spec, skill_slug }),
    });
    if (!response.ok) {
        throw new Error(`从 skills.sh 导入 Skill 失败: ${response.statusText}`);
    }
    const result: ApiResponse<SkillDetail> = await response.json();
    return result.data;
};
