/**
 * Workspace Live Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/app/workspace-live
 * [OUTPUT]: 对外提供 useWorkspaceLiveStore、正文 revision 投影与 owner reset
 * [POS]: store 层的 owner-scoped workspace 实时状态，驱动文件树/编辑器动态反馈
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
              content_revision: null,
              diff_stats: null,
              updated_at: nextUpdatedAt,
            },
            ...state.recent_events,
          ].slice(0, 24),
          file_states: restFileStates,
        };
      }

      const previousFileState = state.file_states[key];
      const nextLiveContent = resolveLiveContent(previousFileState?.live_content, event);
      const nextContentRevision = resolveContentRevision(
        previousFileState?.content_revision,
        previousFileState?.version,
        event,
      );

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
            content_revision: nextContentRevision,
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
            content_revision: nextContentRevision,
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

/** Auth owner 变化时清空旧连接留下的文件事件与内容快照。 */
export function resetWorkspaceLiveOwnerScope(): void {
  useWorkspaceLiveStore.setState({
    recent_events: [],
    file_states: {},
  });
}

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

function resolveContentRevision(
  previousRevision: string | null | undefined,
  previousVersion: number | undefined,
  event: WorkspaceLiveEvent,
): string | null | undefined {
  if (typeof event.content_revision === 'string' && event.content_revision) {
    return event.content_revision;
  }
  if (event.type === 'file_write_start' || previousVersion !== event.version) {
    return null;
  }
  return previousRevision;
}
