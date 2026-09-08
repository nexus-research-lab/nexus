// INPUT: Workspace 菜单调用元素、原始位置、文件身份及创建/重命名草稿。
// OUTPUT: 无菜单尺寸或宿主视觉判断的本地交互状态与 Prompt 默认值。
// POS: Workspace 交互模型；浮层几何由 shared/ui/overlay 负责。
import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceContextMenuState {
  anchor: HTMLElement | null;
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

export function createWorkspacePrompt(
  entryType: "file" | "directory",
  parentPath: string | null,
): WorkspacePromptState {
  return { ...CREATE_PROMPT_DEFAULTS[entryType], parentPath };
}
