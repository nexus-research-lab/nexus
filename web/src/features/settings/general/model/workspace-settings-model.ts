type StateRootPlaceholderKey =
  | "settings.general.state_root_placeholder_macos"
  | "settings.general.state_root_placeholder_posix"
  | "settings.general.state_root_placeholder_windows";

interface DesktopStateRootSnapshot {
  current_path: string;
}

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

export function getStateRootPlaceholderKey(
  platform?: string,
): StateRootPlaceholderKey {
  const normalizedPlatform = platform?.trim().toLowerCase();
  if (normalizedPlatform === "windows") {
    return "settings.general.state_root_placeholder_windows";
  }
  if (normalizedPlatform === "macos") {
    return "settings.general.state_root_placeholder_macos";
  }
  return "settings.general.state_root_placeholder_posix";
}

function normalizeWorkspacePath(value?: string): string {
  return value?.trim() ?? "";
}

export function buildStateRootSettingsSnapshot(
  status: DesktopStateRootSnapshot,
): WorkspaceSettingsSnapshot {
  const currentPath = normalizeWorkspacePath(status.current_path);
  return {
    currentPath,
    draftPath: currentPath,
    savedPath: currentPath,
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
  return !busy && draftPath !== "" && draftPath !== snapshot.savedPath;
}
