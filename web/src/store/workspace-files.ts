/**
 * Workspace Files Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/agent/agent
 * [OUTPUT]: 对外提供 useWorkspaceFilesStore
 * [POS]: store 层共享当前 workspace 文件列表，用于跨组件判断文件是否存在
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';

import { getWorkspaceFilesApi } from '@/lib/api/agent/agent-api';
import { WorkspaceFileEntry } from '@/types/agent/agent';

const requestVersionByAgent = new Map<string, number>();

function nextRequestVersion(agentId: string): number {
  const nextVersion = (requestVersionByAgent.get(agentId) ?? 0) + 1;
  requestVersionByAgent.set(agentId, nextVersion);
  return nextVersion;
}

function isCurrentRequest(agentId: string, requestVersion: number): boolean {
  return requestVersionByAgent.get(agentId) === requestVersion;
}

interface WorkspaceFilesStoreState {
  files_by_agent: Record<string, WorkspaceFileEntry[]>;
  // 一次性「打开文件时请求切到的归属 Agent」：消息区点资产带上 workspace_agent_id，
  // workspace 面板消费后切换 selectedAgentId 并清空，避免污染用户的手动切换。
  requested_open_agent_id: string | null;
  set_files: (agentId: string, files: WorkspaceFileEntry[]) => void;
  clear_agent: (agentId: string) => void;
  refresh_files: (agentId: string) => Promise<WorkspaceFileEntry[]>;
  request_open_agent: (agentId: string | null) => void;
}

export const useWorkspaceFilesStore = create<WorkspaceFilesStoreState>()((set) => ({
  files_by_agent: {},
  requested_open_agent_id: null,

  request_open_agent: (agentId) => {
    set({ requested_open_agent_id: agentId ? agentId.trim() || null : null });
  },

  set_files: (agentId, files) => {
    nextRequestVersion(agentId);
    set((state) => ({
      files_by_agent: {
        ...state.files_by_agent,
        [agentId]: files,
      },
    }));
  },

  clear_agent: (agentId) => {
    nextRequestVersion(agentId);
    set((state) => {
      const next = { ...state.files_by_agent };
      delete next[agentId];
      return { files_by_agent: next };
    });
  },

  refresh_files: async (agentId) => {
    const requestVersion = nextRequestVersion(agentId);
    const files = await getWorkspaceFilesApi(agentId);
    if (!isCurrentRequest(agentId, requestVersion)) {
      return files;
    }
    set((state) => ({
      files_by_agent: {
        ...state.files_by_agent,
        [agentId]: files,
      },
    }));
    return files;
  },
}));
