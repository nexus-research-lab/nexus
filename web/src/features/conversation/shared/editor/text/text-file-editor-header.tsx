import { type ComponentType } from "react";
import { Eye, FileText, LoaderCircle, Pencil, Save } from "lucide-react";

import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
  WorkspaceFileToolbarButton,
} from "../workspace-file-preview-chrome";
import {
  workspaceFileKindLabel,
  type WorkspaceFilePreviewKind,
} from "../workspace-file-preview-kind";
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
  embedded?: boolean;
  fileName: string;
  fileType: WorkspaceFilePreviewKind;
  isPreviewFocused?: boolean;
  onSave: () => void;
  onToggleEditing: () => void;
  onTogglePreviewFocus?: () => void;
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
      <LoaderCircle className="h-3 w-3 shrink-0 animate-spin text-primary" />
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
  embedded,
  fileName,
  fileType,
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
            onClick={onToggleEditing}
            title={presentation.editLabel}
          >
            <EditIcon className="h-4 w-4" />
            <span className="max-xl:hidden">{presentation.editLabel}</span>
          </WorkspaceFileToolbarButton>
          <WorkspaceFileToolbarButton
            disabled={presentation.saveDisabled}
            onClick={onSave}
            title={presentation.saveLabel}
          >
            <Save className="h-4 w-4" />
            <span className="max-xl:hidden">{presentation.saveLabel}</span>
          </WorkspaceFileToolbarButton>
        </>
      )}
      embedded={embedded}
      meta={(
        <>
          <span className="flex items-center gap-1">
            <FileText className="h-3 w-3" />
            {workspaceFileKindLabel(fileType)}
          </span>
          <TextEditorSyncStatus presentation={presentation.sync} />
        </>
      )}
      title={fileName}
    />
  );
}
