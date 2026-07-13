export type MemoryDocumentKind = "index" | "topic" | "daily_log";

export type MemoryDocumentType = "user" | "feedback" | "project" | "reference" | "";

export type MemoryLayout = "empty" | "topic" | "daily_log" | "mixed";

export interface MemoryDocument {
  description?: string;
  indexed: boolean;
  kind: MemoryDocumentKind;
  modified_at: string;
  name?: string;
  path: string;
  size: number;
  title: string;
  type?: MemoryDocumentType;
}

export interface MemorySnapshot {
  documents: MemoryDocument[];
  index?: MemoryDocument | null;
  layout: MemoryLayout;
  truncated: boolean;
}
