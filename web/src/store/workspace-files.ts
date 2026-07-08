/**
 * Workspace Files Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/agent/agent
 * [OUTPUT]: 对外提供 useWorkspaceFilesStore
 * [POS]: store 层共享当前 workspace 文件列表，用于跨组件判断文件是否存在
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';

import { getWorkspaceFilesApi } from '@/lib/api/agent-manage-api';
import { WorkspaceFileEntry } from '@/types/agent/agent';

interface WorkspaceFilesStoreState {
  files_by_agent: Record<string, WorkspaceFileEntry[]>;
  set_files: (agentId: string, files: WorkspaceFileEntry[]) => void;
  clear_agent: (agentId: string) => void;
  refresh_files: (agentId: string) => Promise<WorkspaceFileEntry[]>;
}

export const useWorkspaceFilesStore = create<WorkspaceFilesStoreState>()((set) => ({
  files_by_agent: {},

  set_files: (agentId, files) => {
    set((state) => ({
      files_by_agent: {
        ...state.files_by_agent,
        [agentId]: files,
      },
    }));
  },

  clear_agent: (agentId) => {
    set((state) => {
      const next = { ...state.files_by_agent };
      delete next[agentId];
      return { files_by_agent: next };
    });
  },

  refresh_files: async (agentId) => {
    const files = await getWorkspaceFilesApi(agentId);
    set((state) => ({
      files_by_agent: {
        ...state.files_by_agent,
        [agentId]: files,
      },
    }));
    return files;
  },
}));
