// INPUT: 文件元信息、纯展示模型和 exact editor 命令。
// OUTPUT: 下载、聚焦、编辑、保存与外部同步状态工具栏。
// POS: 文本编辑器 Header；只投影可用性，不拥有保存或恢复语义。
import { type ComponentType } from "react";
import { Eye, LoaderCircle, Pencil, Save } from "lucide-react";

import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
  WorkspaceFileToolbarButton,
} from "../workspace-file-preview-chrome";
import type {
  TextEditorEditAction,
  TextEditorSyncPresentation,
  TextFileEditorPresentation,
} from "./text-file-editor-model";

interface IconProps {
  className?: string;
}

interface TextFileEditorHeaderProps {
  agentId: string;
  fileName: string;
  isPreviewFocused: boolean;
  onSave: () => void;
  onToggleEditing: () => void;
  onTogglePreviewFocus: () => void;
  path: string;
  presentation: TextFileEditorPresentation;
}

const EDIT_ACTION_ICONS: Record<
  TextEditorEditAction,
  ComponentType<IconProps>
> = {
  edit: Pencil,
  preview: Eye,
};

function WritingStatus({ label }: { label: string }) {
  return (
    <>
      <LoaderCircle className="h-3 w-3 shrink-0 animate-spin text-primary motion-reduce:animate-none" />
      <span className="truncate">{label}</span>
    </>
  );
}

function SyncedStatus({ label }: { label: string }) {
  return (
    <>
      <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--success)" />
      <span className="truncate">{label}</span>
    </>
  );
}

const SYNC_STATUS_VIEWS: Record<
  TextEditorSyncPresentation["kind"],
  ComponentType<{ label: string }>
> = {
  synced: SyncedStatus,
  writing: WritingStatus,
};

function TextEditorSyncStatus({
  presentation,
}: {
  presentation: TextEditorSyncPresentation | null;
}) {
  if (!presentation) {
    return null;
  }
  const Status = SYNC_STATUS_VIEWS[presentation.kind];
  return <Status label={presentation.label} />;
}

export function TextFileEditorHeader({
  agentId,
  fileName,
  isPreviewFocused,
  onSave,
  onToggleEditing,
  onTogglePreviewFocus,
  path,
  presentation,
}: TextFileEditorHeaderProps) {
  const EditIcon = EDIT_ACTION_ICONS[presentation.editAction];
  return (
    <WorkspaceFilePreviewHeader
      actions={(
        <>
          <WorkspaceFileDownloadButton
            agentId={agentId}
            fileName={fileName}
            path={path}
          />
          <WorkspaceFilePreviewFocusButton
            isPreviewFocused={isPreviewFocused}
            onTogglePreviewFocus={onTogglePreviewFocus}
          />
          <WorkspaceFileToolbarButton
            disabled={presentation.editDisabled}
            onClick={onToggleEditing}
            title={presentation.editLabel}
          >
            <EditIcon className="h-3.5 w-3.5" />
          </WorkspaceFileToolbarButton>
          <WorkspaceFileToolbarButton
            disabled={presentation.saveDisabled}
            onClick={onSave}
            title={presentation.saveLabel}
          >
            <Save className="h-3.5 w-3.5" />
          </WorkspaceFileToolbarButton>
        </>
      )}
      meta={presentation.sync
        ? <TextEditorSyncStatus presentation={presentation.sync} />
        : undefined}
      title={fileName}
    />
  );
}
