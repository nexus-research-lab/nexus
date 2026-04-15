/**
 * Workspace Files Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/agent
 * [OUTPUT]: 对外提供 useWorkspaceFilesStore
 * [POS]: store 层共享当前 workspace 文件列表，用于跨组件判断文件是否存在
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';

import { get_workspace_files_api } from '@/lib/agent-manage-api';
import { WorkspaceFileEntry } from '@/types/agent';

interface WorkspaceFilesStoreState {
  files_by_agent: Record<string, WorkspaceFileEntry[]>;
  set_files: (agent_id: string, files: WorkspaceFileEntry[]) => void;
  clear_agent: (agent_id: string) => void;
  refresh_files: (agent_id: string) => Promise<WorkspaceFileEntry[]>;
}

export const useWorkspaceFilesStore = create<WorkspaceFilesStoreState>()((set) => ({
  files_by_agent: {},

  set_files: (agent_id, files) => {
    set((state) => ({
      files_by_agent: {
        ...state.files_by_agent,
        [agent_id]: files,
      },
    }));
  },

  clear_agent: (agent_id) => {
    set((state) => {
      const next = { ...state.files_by_agent };
      delete next[agent_id];
      return { files_by_agent: next };
    });
  },

  refresh_files: async (agent_id) => {
    const files = await get_workspace_files_api(agent_id);
    set((state) => ({
      files_by_agent: {
        ...state.files_by_agent,
        [agent_id]: files,
      },
    }));
    return files;
  },
}));
