/**
 * Agent Store - 主入口
 *
 * 使用 Zustand 管理 Agent 状态
 *
 * [INPUT]: 依赖 @/lib/api/agent-manage-api 的 Agent API
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
import { create_browser_json_storage } from "@/lib/storage/browser-storage";
import {
  get_agents,
  create_agent_api,
  update_agent_api,
  delete_agent_api,
} from "@/lib/api/agent-manage-api";

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
  delete_agent: (agent_id: string) => Promise<void>;
  update_agent: (agent_id: string, params: UpdateAgentParams) => Promise<void>;
  set_current_agent: (agent_id: string | null) => void;

  // 查询
  get_agent: (agent_id: string) => Agent | undefined;

  // 服务器同步
  load_agents_from_server: () => Promise<void>;
  apply_agent_runtime_status: (status: AgentRuntimeStatus) => void;
}

function build_idle_runtime_status(agent_id: string): AgentRuntimeStatus {
  return {
    agent_id,
    running_task_count: 0,
    status: "idle",
  };
}

let load_agents_inflight: Promise<Agent[]> | null = null;

function run_agent_list_request(): Promise<Agent[]> {
  if (load_agents_inflight) {
    return load_agents_inflight;
  }

  load_agents_inflight = get_agents().finally(() => {
    load_agents_inflight = null;
  });
  return load_agents_inflight;
}

function dispatch_agent_list_updated() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AGENT_LIST_UPDATED_EVENT_NAME));
}

function are_agent_runtime_statuses_equal(
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
          const agent = await create_agent_api(params);
          set((state) => ({
            agents: [agent, ...state.agents],
            agent_runtime_statuses: {
              ...state.agent_runtime_statuses,
              [agent.agent_id]: build_idle_runtime_status(agent.agent_id),
            },
            error: null,
          }));
          dispatch_agent_list_updated();
          console.debug("[AgentStore] Agent created:", agent.agent_id);
          return agent.agent_id;
        } catch (error) {
          console.error("[AgentStore] Failed to create agent:", error);
          set({ error: "Failed to create agent" });
          throw error;
        }
      },

      delete_agent: async (agent_id: string): Promise<void> => {
        try {
          await delete_agent_api(agent_id);
          set((state) => {
            const new_agents = state.agents.filter(
              (a) => a.agent_id !== agent_id,
            );
            const new_current =
              state.current_agent_id === agent_id
                ? new_agents[0]?.agent_id || null
                : state.current_agent_id;
            return {
              agents: new_agents,
              agent_runtime_statuses: Object.fromEntries(
                Object.entries(state.agent_runtime_statuses).filter(
                  ([runtime_agent_id]) => runtime_agent_id !== agent_id,
                ),
              ),
              current_agent_id: new_current,
              error: null,
            };
          });
          dispatch_agent_list_updated();
        } catch (error) {
          console.error("[AgentStore] Failed to delete agent:", error);
          set({ error: "Failed to delete agent" });
        }
      },

      update_agent: async (
        agent_id: string,
        params: UpdateAgentParams,
      ): Promise<void> => {
        try {
          const updated = await update_agent_api(agent_id, params);
          set((state) => ({
            agents: state.agents.map((a) =>
              a.agent_id === agent_id ? updated : a,
            ),
            agent_runtime_statuses: {
              ...state.agent_runtime_statuses,
              [agent_id]:
                state.agent_runtime_statuses[agent_id] ??
                build_idle_runtime_status(agent_id),
            },
            error: null,
          }));
          dispatch_agent_list_updated();
          console.debug("[AgentStore] Agent updated:", agent_id);
        } catch (error) {
          console.error("[AgentStore] Failed to update agent:", error);
          set({ error: "Failed to update agent" });
        }
      },

      set_current_agent: (agent_id: string | null) => {
        set({ current_agent_id: agent_id, error: null });
      },

      // ==================== 查询 ====================

      get_agent: (agent_id: string): Agent | undefined => {
        return get().agents.find((a) => a.agent_id === agent_id);
      },

      // ==================== 服务器同步 ====================

      load_agents_from_server: async (): Promise<void> => {
        try {
          set({ loading: true, error: null });
          const agents = await run_agent_list_request();
          set((state) => ({
            agents,
            agent_runtime_statuses: Object.fromEntries(
              agents.map((agent) => [
                agent.agent_id,
                state.agent_runtime_statuses[agent.agent_id] ??
                  build_idle_runtime_status(agent.agent_id),
              ]),
            ),
            loading: false,
            error: null,
          }));
          console.debug(
            `[AgentStore] Loaded ${agents.length} agents from server`,
          );
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
          const current_status = state.agent_runtime_statuses[status.agent_id];
          if (are_agent_runtime_statuses_equal(current_status, status)) {
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
      storage: create_browser_json_storage(),
      partialize: (state) => ({
        agents: state.agents,
        current_agent_id: state.current_agent_id,
      }),
    },
  ),
);
