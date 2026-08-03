import type { RuntimeSettings } from "@/types/settings/runtime";

type WorkspacePathPlaceholderKey =
  | "settings.general.workspace_path_placeholder_macos"
  | "settings.general.workspace_path_placeholder_posix"
  | "settings.general.workspace_path_placeholder_windows";

export interface WorkspaceSettingsSnapshot {
  currentPath: string;
  draftPath: string;
  restartRequired: boolean;
  savedPath: string;
}

export const EMPTY_WORKSPACE_SETTINGS_SNAPSHOT: WorkspaceSettingsSnapshot = {
  currentPath: "",
  draftPath: "",
  restartRequired: false,
  savedPath: "",
};

export function getWorkspacePathPlaceholderKey(
  platform?: string,
): WorkspacePathPlaceholderKey {
  const normalizedPlatform = platform?.trim().toLowerCase();
  if (normalizedPlatform === "windows") {
    return "settings.general.workspace_path_placeholder_windows";
  }
  if (normalizedPlatform === "macos") {
    return "settings.general.workspace_path_placeholder_macos";
  }
  return "settings.general.workspace_path_placeholder_posix";
}

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
    restartRequired: settings.restart_required === true,
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
  const draftPath = normalizeWorkspacePath(snapshot.draftPath);
  if (busy || draftPath === snapshot.savedPath) {
    return false;
  }
  return snapshot.restartRequired || draftPath !== snapshot.currentPath;
}
