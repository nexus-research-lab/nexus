/**
 * Workspace Live Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/app/workspace-live
 * [OUTPUT]: 对外提供 useWorkspaceLiveStore
 * [POS]: store 层的 workspace 实时状态，驱动文件树/编辑器动态反馈
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

import { create } from 'zustand';

import { WorkspaceActivityItem, WorkspaceLiveEvent, WorkspaceLiveFileState } from '@/types/app/workspace-live';

interface WorkspaceLiveStoreState {
  recent_events: WorkspaceActivityItem[];
  file_states: Record<string, WorkspaceLiveFileState>;
  apply_event: (event: WorkspaceLiveEvent) => void;
  mark_file_seen: (agentId: string, path: string) => void;
  settle_agent_writes: (agentId: string) => void;
  clear_agent: (agentId: string) => void;
}

function buildKey(agentId: string, path: string) {
  return `${agentId}:${path}`;
}

export const useWorkspaceLiveStore = create<WorkspaceLiveStoreState>()((set) => ({
  recent_events: [],
  file_states: {},

  apply_event: (event) => {
    const key = buildKey(event.agent_id, event.path);
    const nextStatus: WorkspaceLiveFileState['status'] =
      event.type === 'file_write_end' ? 'updated' : 'writing';
    const nextUpdatedAt = Date.parse(event.timestamp) || Date.now();

    set((state) => {
      if (event.type === 'file_deleted') {
        const { [key]: _, ...restFileStates } = state.file_states;
        return {
          recent_events: [
            {
              id: `${key}:${event.type}:${event.version}:${nextUpdatedAt}`,
              event_type: event.type,
              agent_id: event.agent_id,
              path: event.path,
              status: 'deleted' as const,
              version: event.version,
              source: event.source,
              live_content: null,
              diff_stats: null,
              updated_at: nextUpdatedAt,
            },
            ...state.recent_events,
          ].slice(0, 24),
          file_states: restFileStates,
        };
      }

      const nextLiveContent = resolveLiveContent(state.file_states[key]?.live_content, event);

      return {
        recent_events: [
          {
            id: `${key}:${event.type}:${event.version}:${nextUpdatedAt}`,
            event_type: event.type,
            agent_id: event.agent_id,
            path: event.path,
            status: nextStatus,
            version: event.version,
            source: event.source,
            live_content: nextLiveContent,
            diff_stats: event.diff_stats,
            updated_at: nextUpdatedAt,
          },
          ...state.recent_events,
        ].slice(0, 24),
        file_states: {
          ...state.file_states,
          [key]: {
            agent_id: event.agent_id,
            path: event.path,
            status: nextStatus,
            version: event.version,
            source: event.source,
            live_content: nextLiveContent,
            diff_stats: event.diff_stats,
            updated_at: nextUpdatedAt,
          },
        },
      };
    });
  },

  mark_file_seen: (agentId, path) => {
    const key = buildKey(agentId, path);

    set((state) => {
      const nextFileStates = { ...state.file_states };
      delete nextFileStates[key];

      return {
        recent_events: [
          ...state.recent_events.filter((item) => !(item.agent_id === agentId && item.path === path)),
        ],
        file_states: nextFileStates,
      };
    });
  },

  settle_agent_writes: (agentId) => {
    const normalizedAgentId = agentId.trim();
    if (!normalizedAgentId) {
      return;
    }

    set((state) => {
      let hasChanges = false;
      const settledAt = Date.now();
      const nextFileStates = Object.fromEntries(
        Object.entries(state.file_states).map(([key, value]) => {
          if (value.agent_id !== normalizedAgentId || value.status !== 'writing') {
            return [key, value];
          }
          hasChanges = true;
          return [
            key,
            {
              ...value,
              status: 'updated' as const,
              updated_at: settledAt,
            },
          ];
        }),
      );

      if (!hasChanges) {
        return state;
      }

      return {
        recent_events: state.recent_events.map((item) => (
          item.agent_id === normalizedAgentId && item.status === 'writing'
            ? { ...item, status: 'updated' as const, updated_at: settledAt }
            : item
        )),
        file_states: nextFileStates,
      };
    });
  },

  clear_agent: (agentId) => {
    set((state) => ({
      recent_events: state.recent_events.filter((item) => item.agent_id !== agentId),
      file_states: Object.fromEntries(
        Object.entries(state.file_states).filter(([, value]) => value.agent_id !== agentId),
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
