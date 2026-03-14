/**
 * Agent API 服务模块
 *
 * [INPUT]: 依赖 @/types/agent, @/types/cost, @/types/api
 * [OUTPUT]: 对外提供 getAgents、createAgent、updateAgent、deleteAgent 等 API 函数
 * [POS]: lib 模块的 Agent API 层，被 agent store 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import {
    Agent,
    AgentNameValidationResult,
    ApiAgent,
    CreateAgentParams,
    UpdateAgentParams,
    WorkspaceFileContent,
    WorkspaceFileEntry,
    WorkspaceEntryMutationResponse,
    WorkspaceEntryRenameResponse,
} from '@/types/agent';
import { AgentCostSummary } from '@/types/cost';
import { ApiResponse } from '@/types/api';
import { getAgentApiBaseUrl } from '@/lib/runtime-config';

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

// ==================== 类型转换 ====================

function transformApiAgent(api_agent: ApiAgent): Agent {
    return {
        agent_id: api_agent.agent_id,
        name: api_agent.name,
        workspace_path: api_agent.workspace_path,
        options: api_agent.options || {},
        created_at: new Date(api_agent.created_at).getTime(),
        status: api_agent.status,
    };
}

// ==================== Agent API ====================

/** 获取所有 Agent 列表 */
export const getAgents = async (): Promise<Agent[]> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`获取 Agent 列表失败: ${response.statusText}`);
    }
    const result: ApiResponse<ApiAgent[]> = await response.json();
    return result.data.map(transformApiAgent);
};

/** 创建 Agent */
export const createAgentApi = async (params: CreateAgentParams): Promise<Agent> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            name: params.name,
            options: params.options || null,
        }),
    });
    if (!response.ok) {
        throw new Error(`创建 Agent 失败: ${response.statusText}`);
    }
    const result: ApiResponse<ApiAgent> = await response.json();
    return transformApiAgent(result.data);
};



/** 更新 Agent */
export const updateAgentApi = async (agent_id: string, params: UpdateAgentParams): Promise<Agent> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            name: params.name,
            options: params.options || null,
        }),
    });
    if (!response.ok) {
        throw new Error(`更新 Agent 失败: ${response.statusText}`);
    }
    const result: ApiResponse<ApiAgent> = await response.json();
    return transformApiAgent(result.data);
};

/** 删除 Agent */
export const deleteAgentApi = async (agent_id: string): Promise<{ success: boolean }> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`删除 Agent 失败: ${response.statusText}`);
    }
    const result: ApiResponse<{ success: boolean }> = await response.json();
    return result.data;
};

/** 校验 Agent 名称 */
export const validateAgentNameApi = async (
    name: string,
    exclude_agent_id?: string
): Promise<AgentNameValidationResult> => {
    const query = new URLSearchParams({ name });
    if (exclude_agent_id) {
        query.set('exclude_agent_id', exclude_agent_id);
    }

    const response = await fetch(`${AGENT_API_BASE_URL}/agents/validate/name?${query.toString()}`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`校验 Agent 名称失败: ${response.statusText}`);
    }
    const result: ApiResponse<AgentNameValidationResult> = await response.json();
    return result.data;
};

export const getWorkspaceFilesApi = async (agent_id: string): Promise<WorkspaceFileEntry[]> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/files`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`获取 Workspace 文件列表失败: ${response.statusText}`);
    }
    const result: ApiResponse<WorkspaceFileEntry[]> = await response.json();
    return result.data;
};

export const getWorkspaceFileContentApi = async (
    agent_id: string,
    path: string
): Promise<WorkspaceFileContent> => {
    const query = new URLSearchParams({ path });
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/file?${query.toString()}`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`读取 Workspace 文件失败: ${response.statusText}`);
    }
    const result: ApiResponse<WorkspaceFileContent> = await response.json();
    return result.data;
};

export const updateWorkspaceFileContentApi = async (
    agent_id: string,
    path: string,
    content: string
): Promise<WorkspaceFileContent> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/file`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, content }),
    });
    if (!response.ok) {
        throw new Error(`更新 Workspace 文件失败: ${response.statusText}`);
    }
    const result: ApiResponse<WorkspaceFileContent> = await response.json();
    return result.data;
};

export const createWorkspaceEntryApi = async (
    agent_id: string,
    path: string,
    entry_type: 'file' | 'directory',
    content: string = ''
): Promise<WorkspaceEntryMutationResponse> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, entry_type, content }),
    });
    if (!response.ok) {
        throw new Error(`创建 Workspace 条目失败: ${response.statusText}`);
    }
    const result: ApiResponse<WorkspaceEntryMutationResponse> = await response.json();
    return result.data;
};

export const renameWorkspaceEntryApi = async (
    agent_id: string,
    path: string,
    new_path: string
): Promise<WorkspaceEntryRenameResponse> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, new_path }),
    });
    if (!response.ok) {
        throw new Error(`重命名 Workspace 条目失败: ${response.statusText}`);
    }
    const result: ApiResponse<WorkspaceEntryRenameResponse> = await response.json();
    return result.data;
};

export const deleteWorkspaceEntryApi = async (
    agent_id: string,
    path: string
): Promise<WorkspaceEntryMutationResponse> => {
    const query = new URLSearchParams({ path });
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry?${query.toString()}`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`删除 Workspace 条目失败: ${response.statusText}`);
    }
    const result: ApiResponse<WorkspaceEntryMutationResponse> = await response.json();
    return result.data;
};

export const getAgentCostSummaryApi = async (agent_id: string): Promise<AgentCostSummary> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}/cost/summary`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`获取 Agent 成本失败: ${response.statusText}`);
    }
    const result: ApiResponse<AgentCostSummary> = await response.json();
    return result.data;
};
