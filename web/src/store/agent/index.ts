/**
 * Agent Store - 主入口
 *
 * 使用 Zustand 管理 Agent 状态
 *
 * [INPUT]: 依赖 @/lib/api/agent/agent-api 的 Agent API
 * [OUTPUT]: 对外提供 useAgentStore
 * [POS]: store 模块的 Agent 管理，被侧边栏和 Agent 设置页消费
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";
import {
  Agent,
  AgentRuntimeStatus,
  CreateAgentParams,
  UpdateAgentParams,
} from "@/types/agent/agent";
import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";
import {
  getAgents,
  createAgentApi,
  updateAgentApi,
  deleteAgentApi,
} from "@/lib/api/agent/agent-api";

export const AGENT_LIST_UPDATED_EVENT_NAME = "nexus:agent-list-updated";

// ==================== Store 类型 ====================

export interface AgentStoreState {
  // 数据
  agents: Agent[];
  agent_runtime_statuses: Record<string, AgentRuntimeStatus>;
  current_agent_id: string | null;

  // UI 状态
  loading: boolean;
  error: string | null;

  // Agent 操作
  create_agent: (params: CreateAgentParams) => Promise<string>;
  delete_agent: (agentId: string) => Promise<void>;
  update_agent: (agentId: string, params: UpdateAgentParams) => Promise<void>;
  set_current_agent: (agentId: string | null) => void;

  // 查询
  get_agent: (agentId: string) => Agent | undefined;

  // 服务器同步
  load_agents_from_server: () => Promise<void>;
  apply_agent_runtime_status: (status: AgentRuntimeStatus) => void;
}

function buildIdleRuntimeStatus(agentId: string): AgentRuntimeStatus {
  return {
    agent_id: agentId,
    running_task_count: 0,
    status: "idle",
  };
}

let loadAgentsInflight: Promise<Agent[]> | null = null;

function runAgentListRequest(): Promise<Agent[]> {
  if (loadAgentsInflight) {
    return loadAgentsInflight;
  }

  loadAgentsInflight = getAgents().finally(() => {
    loadAgentsInflight = null;
  });
  return loadAgentsInflight;
}

function dispatchAgentListUpdated() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AGENT_LIST_UPDATED_EVENT_NAME));
}

function areAgentRuntimeStatusesEqual(
  left: AgentRuntimeStatus | undefined,
  right: AgentRuntimeStatus,
): boolean {
  if (!left) {
    return false;
  }

  return (
    left.agent_id === right.agent_id &&
    left.status === right.status &&
    left.running_task_count === right.running_task_count
  );
}

// ==================== Store 创建 ====================

export const useAgentStore = create<AgentStoreState>()(
  persist(
    (set, get) => ({
      // 初始状态
      agents: [],
      agent_runtime_statuses: {},
      current_agent_id: null,
      loading: false,
      error: null,

      // ==================== Agent 操作 ====================

      create_agent: async (params: CreateAgentParams): Promise<string> => {
        try {
          const agent = await createAgentApi(params);
          set((state) => ({
            agents: [agent, ...state.agents],
            agent_runtime_statuses: {
              ...state.agent_runtime_statuses,
              [agent.agent_id]: buildIdleRuntimeStatus(agent.agent_id),
            },
            error: null,
          }));
          dispatchAgentListUpdated();
          return agent.agent_id;
        } catch (error) {
          console.error("[AgentStore] Failed to create agent:", error);
          set({ error: "Failed to create agent" });
          throw error;
        }
      },

      delete_agent: async (agentId: string): Promise<void> => {
        try {
          await deleteAgentApi(agentId);
          set((state) => {
            const newAgents = state.agents.filter(
              (a) => a.agent_id !== agentId,
            );
            const newCurrent =
              state.current_agent_id === agentId
                ? newAgents[0]?.agent_id || null
                : state.current_agent_id;
            return {
              agents: newAgents,
              agent_runtime_statuses: Object.fromEntries(
                Object.entries(state.agent_runtime_statuses).filter(
                  ([runtimeAgentId]) => runtimeAgentId !== agentId,
                ),
              ),
              current_agent_id: newCurrent,
              error: null,
            };
          });
          dispatchAgentListUpdated();
        } catch (error) {
          console.error("[AgentStore] Failed to delete agent:", error);
          set({ error: "Failed to delete agent" });
        }
      },

      update_agent: async (
        agentId: string,
        params: UpdateAgentParams,
      ): Promise<void> => {
        try {
          const updated = await updateAgentApi(agentId, params);
          set((state) => ({
            agents: state.agents.map((a) =>
              a.agent_id === agentId ? updated : a,
            ),
            agent_runtime_statuses: {
              ...state.agent_runtime_statuses,
              [agentId]:
                state.agent_runtime_statuses[agentId] ??
                buildIdleRuntimeStatus(agentId),
            },
            error: null,
          }));
          dispatchAgentListUpdated();
        } catch (error) {
          console.error("[AgentStore] Failed to update agent:", error);
          set({ error: "Failed to update agent" });
        }
      },

      set_current_agent: (agentId: string | null) => {
        set({ current_agent_id: agentId, error: null });
      },

      // ==================== 查询 ====================

      get_agent: (agentId: string): Agent | undefined => {
        return get().agents.find((a) => a.agent_id === agentId);
      },

      // ==================== 服务器同步 ====================

      load_agents_from_server: async (): Promise<void> => {
        try {
          set({ loading: true, error: null });
          const agents = await runAgentListRequest();
          set((state) => ({
            agents,
            agent_runtime_statuses: Object.fromEntries(
              agents.map((agent) => [
                agent.agent_id,
                state.agent_runtime_statuses[agent.agent_id] ??
                  buildIdleRuntimeStatus(agent.agent_id),
              ]),
            ),
            loading: false,
            error: null,
          }));
        } catch (err) {
          console.error("[AgentStore] Failed to load agents:", err);
          set({
            loading: false,
            error: err instanceof Error ? err.message : "Unknown error",
          });
        }
      },

      apply_agent_runtime_status: (status: AgentRuntimeStatus): void => {
        set((state) => {
          const currentStatus = state.agent_runtime_statuses[status.agent_id];
          if (areAgentRuntimeStatusesEqual(currentStatus, status)) {
            return state;
          }

          return {
            agent_runtime_statuses: {
              ...state.agent_runtime_statuses,
              [status.agent_id]: status,
            },
          };
        });
      },
    }),
    {
      name: "agent-ui-agents",
      storage: createBrowserJsonStorage(),
      partialize: (state) => ({
        current_agent_id: state.current_agent_id,
      }),
    },
  ),
);
