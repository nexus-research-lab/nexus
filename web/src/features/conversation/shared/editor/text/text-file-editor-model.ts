import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";

export type TextEditorBodyMode = "editing" | "html" | "preview" | "streaming";
export type TextEditorEditAction = "edit" | "preview";

export interface TextEditorSyncPresentation {
  kind: "synced" | "writing";
  label: string;
}

export interface TextFileEditorPresentation {
  bodyMode: TextEditorBodyMode;
  editAction: TextEditorEditAction;
  editLabel: string;
  saveDisabled: boolean;
  saveLabel: string;
  sync: TextEditorSyncPresentation | null;
}

interface TextFileEditorPresentationInput {
  fileType: WorkspaceFilePreviewKind;
  isDirty: boolean;
  isEditing: boolean;
  isExternalWriting: boolean;
  isSaving: boolean;
  liveState: WorkspaceLiveFileState | undefined;
  translate: I18nContextValue["t"];
}

interface TextEditorBodyModeInput {
  fileType: WorkspaceFilePreviewKind;
  isEditing: boolean;
  isExternalWriting: boolean;
}

const BODY_MODE_RULES: Array<{
  matches: (input: TextEditorBodyModeInput) => boolean;
  mode: TextEditorBodyMode;
}> = [
  {
    matches: ({ fileType, isExternalWriting }) => (
      isExternalWriting && fileType !== "html"
    ),
    mode: "streaming",
  },
  {
    matches: ({ isEditing }) => isEditing,
    mode: "editing",
  },
  {
    matches: ({ fileType }) => fileType === "html",
    mode: "html",
  },
];

function resolveBodyMode(
  input: TextEditorBodyModeInput,
): TextEditorBodyMode {
  return BODY_MODE_RULES.find((rule) => rule.matches(input))?.mode ?? "preview";
}

function buildSyncedLabel(
  diffStats: WorkspaceLiveFileState["diff_stats"],
  translate: I18nContextValue["t"],
): string {
  if (!diffStats) {
    return translate("workspace_file.synced");
  }
  return translate("workspace_file.synced_with_changes", {
    additions: diffStats.additions,
    deletions: diffStats.deletions,
  });
}

function buildSyncPresentation(
  liveState: WorkspaceLiveFileState | undefined,
  isExternalWriting: boolean,
  translate: I18nContextValue["t"],
): TextEditorSyncPresentation | null {
  // API 写入由保存动作反馈；这里只展示外部写入，避免同一事务出现两套状态。
  if (!liveState || liveState.source === "api") {
    return null;
  }
  if (isExternalWriting) {
    return {
      kind: "writing",
      label: translate("workspace_file.model_writing"),
    };
  }
  return {
    kind: "synced",
    label: buildSyncedLabel(liveState.diff_stats, translate),
  };
}

export function buildTextFileEditorPresentation({
  fileType,
  isDirty,
  isEditing,
  isExternalWriting,
  isSaving,
  liveState,
  translate,
}: TextFileEditorPresentationInput): TextFileEditorPresentation {
  const editAction: TextEditorEditAction = isEditing ? "preview" : "edit";
  return {
    bodyMode: resolveBodyMode({ fileType, isEditing, isExternalWriting }),
    editAction,
    editLabel: translate(editAction === "edit"
      ? "common.edit"
      : "workspace_file.preview"),
    saveDisabled: !isDirty || isSaving || isExternalWriting,
    saveLabel: translate(isSaving ? "common.saving" : "common.save"),
    sync: buildSyncPresentation(liveState, isExternalWriting, translate),
  };
}
