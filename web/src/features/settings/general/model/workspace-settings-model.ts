import type { RuntimeSettings } from "@/types/settings/runtime";

export interface WorkspaceSettingsSnapshot {
  currentPath: string;
  draftPath: string;
  savedPath: string;
}

export const EMPTY_WORKSPACE_SETTINGS_SNAPSHOT: WorkspaceSettingsSnapshot = {
  currentPath: "",
  draftPath: "",
  savedPath: "",
};

function normalizeWorkspacePath(value?: string): string {
  return value?.trim() ?? "";
}

export function buildWorkspaceSettingsSnapshot(
  settings: RuntimeSettings,
): WorkspaceSettingsSnapshot {
  const savedPath = normalizeWorkspacePath(settings.workspace_path);
  return {
    currentPath: normalizeWorkspacePath(settings.current_workspace_path),
    draftPath: savedPath,
    savedPath,
  };
}

export function replaceWorkspaceDraft(
  snapshot: WorkspaceSettingsSnapshot,
  draftPath: string,
): WorkspaceSettingsSnapshot {
  return { ...snapshot, draftPath };
}

export function canSaveWorkspaceSettings(
  snapshot: WorkspaceSettingsSnapshot,
  busy: boolean,
): boolean {
  return !busy && normalizeWorkspacePath(snapshot.draftPath) !== snapshot.savedPath;
}
