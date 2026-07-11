import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";

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

const EDIT_ACTION_COPY: Record<
  TextEditorEditAction,
  { label: string }
> = {
  edit: { label: "编辑" },
  preview: { label: "预览" },
};

function resolveBodyMode(
  input: TextEditorBodyModeInput,
): TextEditorBodyMode {
  return BODY_MODE_RULES.find((rule) => rule.matches(input))?.mode ?? "preview";
}

function buildSyncedLabel(
  diffStats: WorkspaceLiveFileState["diff_stats"],
): string {
  if (!diffStats) {
    return "已同步最新内容";
  }
  return `已同步最新内容 · +${diffStats.additions} -${diffStats.deletions}`;
}

function buildSyncPresentation(
  liveState: WorkspaceLiveFileState | undefined,
  isExternalWriting: boolean,
): TextEditorSyncPresentation | null {
  // API 写入由保存动作反馈；这里只展示外部写入，避免同一事务出现两套状态。
  if (!liveState || liveState.source === "api") {
    return null;
  }
  if (isExternalWriting) {
    return { kind: "writing", label: "模型正在实时写入该文件" };
  }
  return {
    kind: "synced",
    label: buildSyncedLabel(liveState.diff_stats),
  };
}

export function buildTextFileEditorPresentation({
  fileType,
  isDirty,
  isEditing,
  isExternalWriting,
  isSaving,
  liveState,
}: TextFileEditorPresentationInput): TextFileEditorPresentation {
  const editAction: TextEditorEditAction = isEditing ? "preview" : "edit";
  return {
    bodyMode: resolveBodyMode({ fileType, isEditing, isExternalWriting }),
    editAction,
    editLabel: EDIT_ACTION_COPY[editAction].label,
    saveDisabled: !isDirty || isSaving || isExternalWriting,
    saveLabel: isSaving ? "保存中" : "保存",
    sync: buildSyncPresentation(liveState, isExternalWriting),
  };
}
