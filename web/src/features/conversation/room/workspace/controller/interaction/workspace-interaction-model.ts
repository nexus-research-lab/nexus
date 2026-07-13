import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceContextMenuState {
  entry: WorkspaceFileEntry | null;
  position: { x: number; y: number } | null;
}

export type WorkspaceCreateMode = "create-file" | "create-directory";

export type WorkspacePromptState =
  | { defaultValue: string; mode: WorkspaceCreateMode; parentPath: string | null }
  | { defaultValue: string; entry: WorkspaceFileEntry; mode: "rename" }
  | null;

const CREATE_PROMPT_DEFAULTS: Record<
  "file" | "directory",
  { defaultValue: string; mode: WorkspaceCreateMode }
> = {
  directory: { defaultValue: "new-folder", mode: "create-directory" },
  file: { defaultValue: "untitled.txt", mode: "create-file" },
};

const MENU_HEIGHT_BY_TARGET = {
  directory: 178,
  file: 102,
  root: 106,
} as const;

export function createWorkspacePrompt(
  entryType: "file" | "directory",
  parentPath: string | null,
): WorkspacePromptState {
  return { ...CREATE_PROMPT_DEFAULTS[entryType], parentPath };
}

export function resolveWorkspaceMenuPosition(
  clientPosition: { x: number; y: number },
  viewport: { height: number; width: number },
  entry: WorkspaceFileEntry | null,
): { x: number; y: number } {
  const target = entry ? (entry.is_dir ? "directory" : "file") : "root";
  return {
    x: Math.max(0, Math.min(clientPosition.x, viewport.width - 180)),
    y: Math.max(
      0,
      Math.min(clientPosition.y, viewport.height - MENU_HEIGHT_BY_TARGET[target]),
    ),
  };
}
