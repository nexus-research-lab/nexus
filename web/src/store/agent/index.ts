/**
 * Agent Store - 主入口
 *
 * 使用 Zustand 管理 Agent 状态
 *
 * [INPUT]: Agent API、当前 owner scope 与创建领域 journal
 * [OUTPUT]: useAgentStore，以及创建/删除对账、兼容加载与 owner reset 入口
 * [POS]: owner-scoped Agent 目录与副作用恢复边界；目录刷新不解锁 pending 创建。
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";
import {
  Agent,
  CreateAgentParams,
  UpdateAgentParams,
} from "@/types/agent/agent";
import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";
import {
  getAgents,
  createAgentApi,
  getAgentCreationRequestApi,
  updateAgentApi,
  deleteAgentApi,
} from "@/lib/api/agent/agent-api";
import { ApiRequestError } from "@/lib/api/core/http-error";
import { getErrorMessage } from "@/lib/error-message";
import type { FailureCore } from "@/types/generated/protocol";

import {
  AgentCreationCoordinationUnavailableError,
  clearAgentCreationJournal,
  createAgentCreationRequestId,
  readAgentCreationJournal,
  withAgentCreationJournalLock,
  writeAgentCreationJournal,
} from "./agent-creation-journal";

export const AGENT_LIST_UPDATED_EVENT_NAME = "nexus:agent-list-updated";

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
  delete_agent: (agentId: string) => Promise<void>;
  update_agent: (agentId: string, params: UpdateAgentParams) => Promise<void>;
  set_current_agent: (agentId: string | null) => void;

  // 查询
  get_agent: (agentId: string) => Agent | undefined;

  // 服务器同步
  load_agents_from_server: () => Promise<void>;
  reconcile_agents_from_server: () => Promise<boolean>;
}

let agentOwnerScopeRevision = 0;
let activeAgentOwnerScope: string | null = null;
let loadAgentsInflight: {
  promise: Promise<Agent[]>;
  revision: number;
} | null = null;

function runAgentListRequest(): Promise<Agent[]> {
  const revision = agentOwnerScopeRevision;
  if (loadAgentsInflight?.revision === revision) {
    return loadAgentsInflight.promise;
  }

  const request = getAgents().finally(() => {
    if (loadAgentsInflight?.promise === request) {
      loadAgentsInflight = null;
    }
  });
  loadAgentsInflight = { promise: request, revision };
  return request;
}

function ownerScopeIsCurrent(revision: number): boolean {
  return revision === agentOwnerScopeRevision;
}

function ownerScopeChangedError(): Error {
  return new Error("登录账号已变化；已忽略上一个账号的 Agent 操作结果");
}

function dispatchAgentListUpdated() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AGENT_LIST_UPDATED_EVENT_NAME));
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
        const ownerRevision = agentOwnerScopeRevision;
        const ownerScope = activeAgentOwnerScope;
        try {
          return await withAgentCreationJournalLock(ownerScope, async () => {
            assertAgentCreationOwnerScope(ownerScope, ownerRevision);
            const snapshot = readAgentCreationJournal(ownerScope);
            if (!snapshot.available) {
              throw agentCreationClientError(
                "agent.creation_journal_unavailable",
                "not_applied",
                "当前无法安全保留创建记录",
              );
            }

            let creationRequestId = snapshot.entry?.requestId ?? null;
            if (creationRequestId) {
              const result = await getAgentCreationRequestApi(creationRequestId);
              assertAgentCreationOwnerScope(ownerScope, ownerRevision);
              if (result.creationRequestId !== creationRequestId) {
                throw agentCreationClientError(
                  "agent.creation_receipt_invalid",
                  "unknown",
                  "Agent 创建记录与当前请求不匹配",
                );
              }
              if (result.status === "committed") {
                if (!result.agent) {
                  throw agentCreationClientError(
                    "agent.creation_projection_incomplete",
                    "committed",
                    "Agent 已创建，但页面还没有拿到完整结果",
                  );
                }
                recordCreatedAgent(result.agent);
                if (!clearAgentCreationJournal(ownerScope)) {
                  throw agentCreationClientError(
                    "agent.creation_journal_cleanup_failed",
                    "committed",
                    "Agent 已创建，但本地创建记录还没有收口",
                  );
                }
                dispatchAgentListUpdated();
                return result.agent.agent_id;
              }
              if (result.status === "deleted" || result.status === "failed") {
                if (!clearAgentCreationJournal(ownerScope)) {
                  throw agentCreationClientError(
                    "agent.creation_journal_unavailable",
                    "not_applied",
                    "已确认上次创建没有可用结果，但本地记录无法安全更新",
                  );
                }
                throw agentCreationClientError(
                  result.status === "deleted"
                    ? "agent.creation_result_deleted"
                    : "agent.creation_failed",
                  "not_applied",
                  result.status === "deleted"
                    ? "上次创建的 Agent 之后已被删除，本次没有重新创建"
                    : "上次 Agent 创建没有完成",
                );
              }
              if (result.status === "pending") {
                throw agentCreationClientError(
                  "agent.creation_in_progress",
                  "accepted",
                  "Agent 创建请求已受理，但还没有完成",
                );
              }
              if (result.status !== "not_found") {
                throw agentCreationClientError(
                  "agent.creation_receipt_invalid",
                  "unknown",
                  "Agent 创建记录状态无法识别",
                );
              }
            } else {
              creationRequestId = createAgentCreationRequestId();
              if (!creationRequestId || !writeAgentCreationJournal(ownerScope, {
                requestId: creationRequestId,
                status: "pending",
              })) {
                throw agentCreationClientError(
                  "agent.creation_journal_unavailable",
                  "not_applied",
                  "当前无法安全保留创建记录",
                );
              }
            }

            if (!writeAgentCreationJournal(ownerScope, {
              requestId: creationRequestId,
              status: "pending",
            })) {
              throw agentCreationClientError(
                "agent.creation_journal_unavailable",
                "not_applied",
                "当前无法安全更新创建记录",
              );
            }

            let agent: Agent;
            try {
              agent = await createAgentApi({
                ...params,
                creation_request_id: creationRequestId,
              });
            } catch (error) {
              if (ownerScopeIsCurrent(ownerRevision) && activeAgentOwnerScope === ownerScope) {
                writeAgentCreationJournal(ownerScope, {
                  requestId: creationRequestId,
                  status: "unconfirmed",
                });
                if (isTerminalAgentCreationFailure(error)) {
                  clearAgentCreationJournal(ownerScope);
                }
              }
              throw error;
            }
            assertAgentCreationOwnerScope(ownerScope, ownerRevision);
            recordCreatedAgent(agent);
            if (!clearAgentCreationJournal(ownerScope)) {
              throw agentCreationClientError(
                "agent.creation_journal_cleanup_failed",
                "committed",
                "Agent 已创建，但本地创建记录还没有收口",
              );
            }
            dispatchAgentListUpdated();
            return agent.agent_id;
          });
        } catch (error) {
          console.error("[AgentStore] Failed to create agent:", error);
          if (ownerScopeIsCurrent(ownerRevision)) {
            set({ error: "Failed to create agent" });
          }
          if (error instanceof AgentCreationCoordinationUnavailableError) {
            throw agentCreationClientError(
              "agent.creation_coordination_unavailable",
              "not_applied",
              error.message,
            );
          }
          throw error;
        }
      },

      delete_agent: async (agentId: string): Promise<void> => {
        const ownerRevision = agentOwnerScopeRevision;
        try {
          await deleteAgentApi(agentId);
          if (!ownerScopeIsCurrent(ownerRevision)) {
            throw ownerScopeChangedError();
          }
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
              current_agent_id: newCurrent,
              error: null,
            };
          });
          dispatchAgentListUpdated();
        } catch (error) {
          console.error("[AgentStore] Failed to delete agent:", error);
          if (ownerScopeIsCurrent(ownerRevision)) {
            set({ error: "Failed to delete agent" });
          }
          throw error;
        }
      },

      update_agent: async (
        agentId: string,
        params: UpdateAgentParams,
      ): Promise<void> => {
        const ownerRevision = agentOwnerScopeRevision;
        try {
          const updated = await updateAgentApi(agentId, params);
          if (!ownerScopeIsCurrent(ownerRevision)) {
            throw ownerScopeChangedError();
          }
          set((state) => ({
            agents: state.agents.map((a) =>
              a.agent_id === agentId ? updated : a,
            ),
            error: null,
          }));
          dispatchAgentListUpdated();
        } catch (error) {
          console.error("[AgentStore] Failed to update agent:", error);
          if (ownerScopeIsCurrent(ownerRevision)) {
            set({ error: "Failed to update agent" });
          }
          throw error;
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
        await get().reconcile_agents_from_server();
      },

      reconcile_agents_from_server: async (): Promise<boolean> => {
        const ownerRevision = agentOwnerScopeRevision;
        try {
          set({ loading: true, error: null });
          const agents = await runAgentListRequest();
          if (!ownerScopeIsCurrent(ownerRevision)) {
            return false;
          }
          set({
            agents,
            loading: false,
            error: null,
          });
          return true;
        } catch (err) {
          console.error("[AgentStore] Failed to load agents:", err);
          if (!ownerScopeIsCurrent(ownerRevision)) {
            return false;
          }
          set({
            loading: false,
            error: getErrorMessage(err, "Agent 列表暂时无法更新"),
          });
          return false;
        }
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

/** Auth owner 变化时同步清空目录，并让旧 owner 的迟到请求失效。 */
export function setAgentOwnerScope(ownerScope: string | null): void {
  activeAgentOwnerScope = ownerScope;
}

export function resetAgentOwnerScope(ownerScope: string | null = null): void {
  agentOwnerScopeRevision += 1;
  activeAgentOwnerScope = ownerScope;
  loadAgentsInflight = null;
  useAgentStore.setState({
    agents: [],
    current_agent_id: null,
    error: null,
    loading: false,
  });
}

function assertAgentCreationOwnerScope(
  ownerScope: string | null,
  ownerRevision: number,
): asserts ownerScope is string {
  if (
    !ownerScope
    || !ownerScopeIsCurrent(ownerRevision)
    || activeAgentOwnerScope !== ownerScope
  ) {
    throw ownerScopeChangedError();
  }
}

function recordCreatedAgent(agent: Agent): void {
  useAgentStore.setState((state) => ({
    agents: [agent, ...state.agents.filter((item) => item.agent_id !== agent.agent_id)],
    error: null,
  }));
}

function agentCreationClientError(
  code: string,
  effect: "accepted" | "committed" | "not_applied" | "unknown",
  message: string,
): ApiRequestError {
  const failure: FailureCore = {
    category: effect === "not_applied"
      ? "unavailable"
      : effect === "accepted"
        ? "conflict"
        : "internal",
    code,
    effect,
    version: 1,
  };
  return new ApiRequestError(message, 409, failure);
}

function isTerminalAgentCreationFailure(error: unknown): boolean {
  if (!(error instanceof ApiRequestError) || error.failure?.version !== 1) {
    return false;
  }
  return error.failure.code === "agent.creation_failed"
    || error.failure.code === "agent.creation_result_deleted";
}
