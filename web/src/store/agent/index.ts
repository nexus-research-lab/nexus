/**
 * Agent Store - 主入口
 *
 * 使用 Zustand 管理 Agent 状态
 *
 * [INPUT]: 依赖 @/lib/agent-manage-api 的 Agent API
 * [OUTPUT]: 对外提供 useAgentStore
 * [POS]: store 模块的 Agent 管理，被侧边栏和 Agent 设置页消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { Agent, CreateAgentParams, UpdateAgentParams } from '@/types/agent';
import { createBrowserJSONStorage } from '@/lib/browser-storage';
import {
    getAgents,
    createAgentApi,
    updateAgentApi,
    deleteAgentApi,
} from '@/lib/agent-manage-api';

// ==================== Store 类型 ====================

export interface AgentStoreState {
    // 数据
    agents: Agent[];
    current_agent_id: string | null;

    // UI 状态
    loading: boolean;
    error: string | null;

    // Agent 操作
    create_agent: (params: CreateAgentParams) => Promise<string>;
    delete_agent: (agent_id: string) => Promise<void>;
    update_agent: (agent_id: string, params: UpdateAgentParams) => Promise<void>;
    set_current_agent: (agent_id: string | null) => void;

    // 查询
    get_agent: (agent_id: string) => Agent | undefined;

    // 服务器同步
    load_agents_from_server: () => Promise<void>;
}

// ==================== Store 创建 ====================

export const useAgentStore = create<AgentStoreState>()(
    persist(
        (set, get) => ({
            // 初始状态
            agents: [],
            current_agent_id: null,
            loading: false,
            error: null,

            // ==================== Agent 操作 ====================

            create_agent: async (params: CreateAgentParams): Promise<string> => {
                try {
                    const agent = await createAgentApi(params);
                    set((state) => ({
                        agents: [agent, ...state.agents],
                        error: null,
                    }));
                    console.debug('[AgentStore] Agent created:', agent.agent_id);
                    return agent.agent_id;
                } catch (error) {
                    console.error('[AgentStore] Failed to create agent:', error);
                    set({ error: 'Failed to create agent' });
                    throw error;
                }
            },

            delete_agent: async (agent_id: string): Promise<void> => {
                try {
                    await deleteAgentApi(agent_id);
                    set((state) => {
                        const new_agents = state.agents.filter(a => a.agent_id !== agent_id);
                        const new_current = state.current_agent_id === agent_id
                            ? (new_agents[0]?.agent_id || null)
                            : state.current_agent_id;
                        return {
                            agents: new_agents,
                            current_agent_id: new_current,
                            error: null,
                        };
                    });
                } catch (error) {
                    console.error('[AgentStore] Failed to delete agent:', error);
                    set({ error: 'Failed to delete agent' });
                }
            },

            update_agent: async (agent_id: string, params: UpdateAgentParams): Promise<void> => {
                try {
                    const updated = await updateAgentApi(agent_id, params);
                    set((state) => ({
                        agents: state.agents.map(a =>
                            a.agent_id === agent_id ? updated : a
                        ),
                        error: null,
                    }));
                    console.debug('[AgentStore] Agent updated:', agent_id);
                } catch (error) {
                    console.error('[AgentStore] Failed to update agent:', error);
                    set({ error: 'Failed to update agent' });
                }
            },

            set_current_agent: (agent_id: string | null) => {
                set({ current_agent_id: agent_id, error: null });
            },

            // ==================== 查询 ====================

            get_agent: (agent_id: string): Agent | undefined => {
                return get().agents.find(a => a.agent_id === agent_id);
            },

            // ==================== 服务器同步 ====================

            load_agents_from_server: async (): Promise<void> => {
                try {
                    set({ loading: true, error: null });
                    const agents = await getAgents();
                    set({ agents, loading: false, error: null });
                    console.debug(`[AgentStore] Loaded ${agents.length} agents from server`);
                } catch (err) {
                    console.error('[AgentStore] Failed to load agents:', err);
                    set({
                        loading: false,
                        error: err instanceof Error ? err.message : 'Unknown error',
                    });
                }
            },
        }),
        {
            name: 'agent-ui-agents',
            storage: createBrowserJSONStorage(),
            partialize: (state) => ({
                agents: state.agents,
                current_agent_id: state.current_agent_id,
            }),
        }
    )
);
