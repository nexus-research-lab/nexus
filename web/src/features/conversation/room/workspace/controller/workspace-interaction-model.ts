import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceContextMenuState {
  position: { x: number; y: number } | null;
  entry: WorkspaceFileEntry | null;
}

export type WorkspaceCreateMode = "create-file" | "create-directory";

export type WorkspacePromptState =
  | { mode: WorkspaceCreateMode; defaultValue: string; parentPath: string | null }
  | { mode: "rename"; entry: WorkspaceFileEntry; defaultValue: string }
  | null;

const CREATE_PROMPT_DEFAULTS: Record<
  "file" | "directory",
  { mode: WorkspaceCreateMode; defaultValue: string }
> = {
  file: { mode: "create-file", defaultValue: "untitled.txt" },
  directory: { mode: "create-directory", defaultValue: "new-folder" },
};

const MENU_HEIGHT_BY_TARGET = {
  root: 106,
  directory: 178,
  file: 102,
} as const;

export function createWorkspacePrompt(
  entryType: "file" | "directory",
  parentPath: string | null,
): WorkspacePromptState {
  return {...CREATE_PROMPT_DEFAULTS[entryType], parentPath};
}

export function resolveWorkspaceMenuPosition(
  clientPosition: { x: number; y: number },
  viewport: { width: number; height: number },
  entry: WorkspaceFileEntry | null,
): { x: number; y: number } {
  const target = entry ? (entry.is_dir ? "directory" : "file") : "root";
  return {
    x: Math.max(0, Math.min(clientPosition.x, viewport.width - 180)),
    y: Math.max(0, Math.min(clientPosition.y, viewport.height - MENU_HEIGHT_BY_TARGET[target])),
  };
}
