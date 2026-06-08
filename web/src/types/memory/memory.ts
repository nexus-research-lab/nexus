export interface MemoryField {
  key: string;
  value: string;
}

export interface MemoryItem {
  entry_id: string;
  path: string;
  kind: string;
  category?: string;
  title: string;
  content: string;
  status: string;
  priority?: string;
  source?: string;
  scope?: string;
  session_key?: string;
  round_id?: string;
  access_count: number;
  score?: number;
  created_at: string;
  fields?: MemoryField[];
}

export interface MemoryStats {
  total: number;
  by_status: Record<string, number>;
  by_kind: Record<string, number>;
  by_scope: Record<string, number>;
  candidate: number;
  accessed: number;
  checkpointed: number;
}

export interface MemoryCleanupResult {
  removed_session_files: number;
  removed_checkpoints: number;
  removed_empty_diaries: number;
  removed_files?: string[];
}

export interface MemoryInjection {
  stable_system_context?: string;
  dynamic_user_context?: string;
  items: MemoryItem[];
}

export interface MemoryWriteInput {
  kind?: string;
  category?: string;
  title?: string;
  content?: string;
  status?: string;
  priority?: string;
  source?: string;
  scope?: string;
  fields?: MemoryField[];
}
