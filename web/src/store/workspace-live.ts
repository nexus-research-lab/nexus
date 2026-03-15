/**
 * Workspace Live Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/workspace-live
 * [OUTPUT]: 对外提供 useWorkspaceLiveStore
 * [POS]: store 层的 workspace 实时状态，驱动文件树/编辑器动态反馈
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';

import { WorkspaceActivityItem, WorkspaceLiveEvent, WorkspaceLiveFileState } from '@/types/workspace-live';

interface WorkspaceLiveStoreState {
  recentEvents: WorkspaceActivityItem[];
  fileStates: Record<string, WorkspaceLiveFileState>;
  applyEvent: (event: WorkspaceLiveEvent) => void;
  markFileSeen: (agentId: string, path: string) => void;
  clearAgent: (agentId: string) => void;
}

function buildKey(agentId: string, path: string) {
  return `${agentId}:${path}`;
}

export const useWorkspaceLiveStore = create<WorkspaceLiveStoreState>()((set) => ({
  recentEvents: [],
  fileStates: {},

  applyEvent: (event) => {
    const key = buildKey(event.agent_id, event.path);
    const nextStatus: WorkspaceLiveFileState['status'] =
      event.type === 'file_write_end' ? 'updated' : 'writing';
    const nextUpdatedAt = Date.parse(event.timestamp) || Date.now();

    set((state) => {
      const nextLiveContent = resolveLiveContent(state.fileStates[key]?.liveContent, event);

      return {
        recentEvents: [
          {
            id: `${key}:${event.type}:${event.version}:${nextUpdatedAt}`,
            eventType: event.type,
            agentId: event.agent_id,
            path: event.path,
            status: nextStatus,
            version: event.version,
            source: event.source,
            liveContent: nextLiveContent,
            diffStats: event.diff_stats,
            updatedAt: nextUpdatedAt,
          },
          ...state.recentEvents,
        ].slice(0, 24),
        fileStates: {
          ...state.fileStates,
          [key]: {
            agentId: event.agent_id,
            path: event.path,
            status: nextStatus,
            version: event.version,
            source: event.source,
            liveContent: nextLiveContent,
            diffStats: event.diff_stats,
            updatedAt: nextUpdatedAt,
          },
        },
      };
    });
  },

  markFileSeen: (agentId, path) => {
    const key = buildKey(agentId, path);

    set((state) => {
      const nextFileStates = { ...state.fileStates };
      delete nextFileStates[key];

      return {
        recentEvents: [
          ...state.recentEvents.filter((item) => !(item.agentId === agentId && item.path === path)),
        ],
        fileStates: nextFileStates,
      };
    });
  },

  clearAgent: (agentId) => {
    set((state) => ({
      recentEvents: state.recentEvents.filter((item) => item.agentId !== agentId),
      fileStates: Object.fromEntries(
        Object.entries(state.fileStates).filter(([, value]) => value.agentId !== agentId),
      ),
    }));
  },
}));

function resolveLiveContent(
  previousContent: string | null | undefined,
  event: WorkspaceLiveEvent,
): string | null | undefined {
  if (typeof event.content_snapshot === 'string') {
    return event.content_snapshot;
  }

  if (
    event.type === 'file_write_delta' &&
    typeof event.appended_text === 'string' &&
    typeof previousContent === 'string'
  ) {
    return `${previousContent}${event.appended_text}`;
  }

  return previousContent;
}
