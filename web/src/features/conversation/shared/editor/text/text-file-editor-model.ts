// INPUT: 文本文件可用性、编辑/保存/实时写入状态与本地化函数。
// OUTPUT: 编辑、保存、预览和同步状态的纯展示模型。
// POS: 文本编辑器控制可用性投影；未成功读取时禁用修改入口。
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
  editDisabled: boolean;
  editLabel: string;
  saveDisabled: boolean;
  saveLabel: string;
  sync: TextEditorSyncPresentation | null;
}

interface TextFileEditorPresentationInput {
  commandBusy: boolean;
  fileType: WorkspaceFilePreviewKind;
  isDirty: boolean;
  isEditing: boolean;
  isAvailable: boolean;
  isExternalWriting: boolean;
  isLoading: boolean;
  isSaving: boolean;
  liveState: WorkspaceLiveFileState | undefined;
  revisionReady: boolean;
  saveBlocked: boolean;
  translate: I18nContextValue["t"];
}

interface TextEditorBodyModeInput {
  fileType: WorkspaceFilePreviewKind;
  isDirty: boolean;
  isEditing: boolean;
  isExternalWriting: boolean;
}

const BODY_MODE_RULES: Array<{
  matches: (input: TextEditorBodyModeInput) => boolean;
  mode: TextEditorBodyMode;
}> = [
  {
    matches: ({ isEditing }) => isEditing,
    mode: "editing",
  },
  {
    matches: ({ fileType, isDirty, isExternalWriting }) => (
      isExternalWriting && !isDirty && fileType !== "html"
    ),
    mode: "streaming",
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
  isDirty: boolean,
  isExternalWriting: boolean,
  saveBlocked: boolean,
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
  if (isDirty || saveBlocked) {
    return null;
  }
  return {
    kind: "synced",
    label: buildSyncedLabel(liveState.diff_stats, translate),
  };
}

export function buildTextFileEditorPresentation({
  commandBusy,
  fileType,
  isDirty,
  isEditing,
  isAvailable,
  isExternalWriting,
  isLoading,
  isSaving,
  liveState,
  revisionReady,
  saveBlocked,
  translate,
}: TextFileEditorPresentationInput): TextFileEditorPresentation {
  const editAction: TextEditorEditAction = isEditing ? "preview" : "edit";
  return {
    bodyMode: resolveBodyMode({
      fileType,
      isDirty,
      isEditing,
      isExternalWriting,
    }),
    editAction,
    editDisabled: !isEditing
      && (!isAvailable || !revisionReady || isExternalWriting),
    editLabel: translate(editAction === "edit"
      ? "common.edit"
      : "workspace_file.preview"),
    saveDisabled: !isAvailable
      || !revisionReady
      || !isDirty
      || isLoading
      || commandBusy
      || isExternalWriting
      || saveBlocked,
    saveLabel: translate(isSaving ? "common.saving" : "common.save"),
    sync: buildSyncPresentation(
      liveState,
      isDirty,
      isExternalWriting,
      saveBlocked,
      translate,
    ),
  };
}
