// INPUT: 文件元信息、纯展示模型和 exact editor 命令。
// OUTPUT: 共享图标动作与轻量外部同步元数据；状态装饰不重复定义字号或胶囊外观。
// POS: 文本编辑器 Header；只投影可用性，不拥有保存或恢复语义。
import { type ComponentType } from "react";
import { Eye, LoaderCircle, Pencil, Save } from "lucide-react";

import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { WORKSPACE_PANEL_HEADER_ICON_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
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

function TextEditorSyncStatus({
  presentation,
}: {
  presentation: TextEditorSyncPresentation;
}) {
  return (
    <>
      {presentation.kind === "writing" ? (
        <LoaderCircle
          aria-hidden="true"
          className={getUiSpinnerClassName({ size: "xs", tone: "primary" })}
        />
      ) : (
        <span aria-hidden="true" className="h-1.5 w-1.5 shrink-0 rounded-full bg-(--success)" />
      )}
      <span className="truncate">{presentation.label}</span>
    </>
  );
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
            <EditIcon className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
          </WorkspaceFileToolbarButton>
          <WorkspaceFileToolbarButton
            disabled={presentation.saveDisabled}
            onClick={onSave}
            title={presentation.saveLabel}
          >
            <Save className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
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
