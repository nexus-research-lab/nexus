/**
 * Agent API 服务模块
 *
 * [INPUT]: 依赖 @/types/agent 的 Agent 类型定义
 * [OUTPUT]: 对外提供 getAgents、createAgent、updateAgent、deleteAgent 等 API 函数
 * [POS]: lib 模块的 Agent API 层，被 agent store 消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import {
    Agent,
    AgentNameValidationResult,
    ApiAgent,
    CreateAgentParams,
    UpdateAgentParams
} from '@/types/agent';

const AGENT_API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8010/agent/v1';

// ==================== API 响应类型 ====================

interface ApiResponse<T> {
    code: number;
    message: string;
    data: T;
}

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

/** 获取单个 Agent */
export const getAgent = async (agent_id: string): Promise<Agent> => {
    const response = await fetch(`${AGENT_API_BASE_URL}/agents/${agent_id}`, {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' },
    });
    if (!response.ok) {
        throw new Error(`获取 Agent 失败: ${response.statusText}`);
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
