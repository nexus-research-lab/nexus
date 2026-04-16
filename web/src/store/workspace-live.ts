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
  mark_file_seen: (agent_id: string, path: string) => void;
  clear_agent: (agent_id: string) => void;
}

function build_key(agent_id: string, path: string) {
  return `${agent_id}:${path}`;
}

export const useWorkspaceLiveStore = create<WorkspaceLiveStoreState>()((set) => ({
  recent_events: [],
  file_states: {},

  apply_event: (event) => {
    const key = build_key(event.agent_id, event.path);
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

      const nextLiveContent = resolve_live_content(state.file_states[key]?.live_content, event);

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

  mark_file_seen: (agent_id, path) => {
    const key = build_key(agent_id, path);

    set((state) => {
      const next_file_states = { ...state.file_states };
      delete next_file_states[key];

      return {
        recent_events: [
          ...state.recent_events.filter((item) => !(item.agent_id === agent_id && item.path === path)),
        ],
        file_states: next_file_states,
      };
    });
  },

  clear_agent: (agent_id) => {
    set((state) => ({
      recent_events: state.recent_events.filter((item) => item.agent_id !== agent_id),
      file_states: Object.fromEntries(
        Object.entries(state.file_states).filter(([, value]) => value.agent_id !== agent_id),
      ),
    }));
  },
}));

function resolve_live_content(
  previous_content: string | null | undefined,
  event: WorkspaceLiveEvent,
): string | null | undefined {
  if (typeof event.content_snapshot === 'string') {
    return event.content_snapshot;
  }

  if (
    event.type === 'file_write_delta' &&
    typeof event.appended_text === 'string' &&
    typeof previous_content === 'string'
  ) {
    return `${previous_content}${event.appended_text}`;
  }

  return previous_content;
}
