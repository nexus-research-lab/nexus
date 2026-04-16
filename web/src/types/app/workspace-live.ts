/**
 * Workspace Live 类型定义
 *
 * [INPUT]: 无外部依赖
 * [OUTPUT]: 对外提供 WorkspaceLiveEvent / WorkspaceLiveFileState
 * [POS]: types 模块的 workspace 实时事件协议
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 */

export interface WorkspaceDiffStats {
  additions: number;
  deletions: number;
  changed_lines: number;
}

export interface WorkspaceLiveEvent {
  type: 'file_write_start' | 'file_write_delta' | 'file_write_end' | 'file_deleted';
  agent_id: string;
  path: string;
  version: number;
  source: 'agent' | 'api' | 'system' | 'unknown';
  session_key?: string | null;
  tool_use_id?: string | null;
  content_snapshot?: string | null;
  appended_text?: string | null;
  diff_stats?: WorkspaceDiffStats | null;
  timestamp: string;
}

export type WorkspaceEventPayload = WorkspaceLiveEvent;

export interface WorkspaceLiveFileState {
  agent_id: string;
  path: string;
  status: 'idle' | 'writing' | 'updated' | 'deleted';
  version: number;
  source: WorkspaceLiveEvent['source'];
  live_content?: string | null;
  diff_stats?: WorkspaceDiffStats | null;
  updated_at: number;
}

export interface WorkspaceActivityItem extends WorkspaceLiveFileState {
  event_type: WorkspaceLiveEvent['type'];
  id: string;
}
