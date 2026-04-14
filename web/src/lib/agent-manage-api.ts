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
    AgentRuntimeStatus,
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
import { getAgentApiBaseUrl } from '@/config/options';
import { request_api } from '@/lib/http';

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
        avatar: api_agent.avatar ?? null,
        description: api_agent.description ?? null,
        vibe_tags: api_agent.vibe_tags ?? [],
        skills_count: api_agent.skills_count ?? null,
    };
}

// ==================== Agent API ====================

/** 获取所有 Agent 列表 */
export const getAgents = async (): Promise<Agent[]> => {
    const result = await request_api<ApiAgent[]>(`${AGENT_API_BASE_URL}/agents`, {
        method: 'GET',
    });
    return result.map(transformApiAgent);
};

/** 获取 Agent 运行态快照 */
export const getAgentRuntimeStatusesApi = async (): Promise<AgentRuntimeStatus[]> => {
    return request_api<AgentRuntimeStatus[]>(`${AGENT_API_BASE_URL}/agents/runtime/statuses`, {
        method: 'GET',
    });
};

/** 创建 Agent */
export const createAgentApi = async (params: CreateAgentParams): Promise<Agent> => {
    const result = await request_api<ApiAgent>(`${AGENT_API_BASE_URL}/agents`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            name: params.name,
            options: params.options || null,
            avatar: params.avatar ?? null,
            description: params.description ?? null,
            vibe_tags: params.vibe_tags ?? [],
        }),
    });
    return transformApiAgent(result);
};



/** 更新 Agent */
export const updateAgentApi = async (agent_id: string, params: UpdateAgentParams): Promise<Agent> => {
    const result = await request_api<ApiAgent>(`${AGENT_API_BASE_URL}/agents/${agent_id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            name: params.name,
            options: params.options || null,
            avatar: params.avatar ?? null,
            description: params.description ?? null,
            vibe_tags: params.vibe_tags ?? [],
        }),
    });
    return transformApiAgent(result);
};

/** 删除 Agent */
export const deleteAgentApi = async (agent_id: string): Promise<{ success: boolean }> => {
    return request_api<{ success: boolean }>(`${AGENT_API_BASE_URL}/agents/${agent_id}`, {
        method: 'DELETE',
    });
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

    return request_api<AgentNameValidationResult>(`${AGENT_API_BASE_URL}/agents/validate/name?${query.toString()}`, {
        method: 'GET',
    });
};

export const getWorkspaceFilesApi = async (agent_id: string): Promise<WorkspaceFileEntry[]> => {
    return request_api<WorkspaceFileEntry[]>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/files`, {
        method: 'GET',
    });
};

export const getWorkspaceFileContentApi = async (
    agent_id: string,
    path: string
): Promise<WorkspaceFileContent> => {
    const query = new URLSearchParams({ path });
    return request_api<WorkspaceFileContent>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/file?${query.toString()}`, {
        method: 'GET',
    });
};

export const updateWorkspaceFileContentApi = async (
    agent_id: string,
    path: string,
    content: string
): Promise<WorkspaceFileContent> => {
    return request_api<WorkspaceFileContent>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/file`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, content }),
    });
};

export const createWorkspaceEntryApi = async (
    agent_id: string,
    path: string,
    entry_type: 'file' | 'directory',
    content: string = ''
): Promise<WorkspaceEntryMutationResponse> => {
    return request_api<WorkspaceEntryMutationResponse>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, entry_type, content }),
    });
};

export const renameWorkspaceEntryApi = async (
    agent_id: string,
    path: string,
    new_path: string
): Promise<WorkspaceEntryRenameResponse> => {
    return request_api<WorkspaceEntryRenameResponse>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path, new_path }),
    });
};

export const deleteWorkspaceEntryApi = async (
    agent_id: string,
    path: string
): Promise<WorkspaceEntryMutationResponse> => {
    const query = new URLSearchParams({ path });
    return request_api<WorkspaceEntryMutationResponse>(`${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/entry?${query.toString()}`, {
        method: 'DELETE',
    });
};

export const getAgentCostSummaryApi = async (agent_id: string): Promise<AgentCostSummary> => {
    return request_api<AgentCostSummary>(`${AGENT_API_BASE_URL}/agents/${agent_id}/cost/summary`, {
        method: 'GET',
    });
};

/** 上传文件到 workspace */
export const uploadWorkspaceFileApi = async (
    agent_id: string,
    file: File,
    path?: string
): Promise<{ path: string; name: string; size: number }> => {
    const formData = new FormData();
    formData.append('file', file);
    if (path) {
        formData.append('path', path);
    }

    const result = await request_api<{ path: string; name: string; size: number }>(
        `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/upload`,
        {
            method: 'POST',
            body: formData,
        }
    );
    return result;
};

/** 获取 workspace 文件下载 URL */
export const getWorkspaceFileDownloadUrl = (agent_id: string, path: string): string => {
    const params = new URLSearchParams({ path });
    return `${AGENT_API_BASE_URL}/agents/${agent_id}/workspace/download?${params.toString()}`;
};
