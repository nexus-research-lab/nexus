"use client";

import { useState } from "react";
import {
  Eye,
  EyeOff,
  FileImage,
  FileText,
  FileWarning,
  LoaderCircle,
} from "lucide-react";

import {
  get_workspace_file_preview_url,
} from "@/lib/api/agent-manage-api";
import { get_workspace_file_external_action_copy } from "@/lib/workspace-file-action";
import { ConversationResizeHandle } from "./conversation-resize-handle";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "./workspace-file-preview-chrome";

interface PreviewFrameProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_resize_start: () => void;
  on_toggle_preview_focus?: () => void;
  path: string;
}

interface BinaryFilePlaceholderProps {
  agent_id: string;
  embedded?: boolean;
  file_name: string;
  is_preview_focused?: boolean;
  on_toggle_preview_focus?: () => void;
  path: string;
}

export function PdfPreview({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: PreviewFrameProps) {
  const [is_loaded, setIsLoaded] = useState(false);
  const preview_url = get_workspace_file_preview_url(agent_id, path);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileText className="h-3 w-3" />
              PDF 预览
            </span>
            {is_loaded ? (
              <span className="flex items-center gap-1 text-(--success)">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                加载中
              </span>
            )}
          </>
        )}
        title={file_name}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        <iframe
          className="h-full w-full"
          sandbox="allow-downloads allow-same-origin"
          src={preview_url}
          title={file_name}
          onLoad={() => setIsLoaded(true)}
        />
      </div>
    </>
  );
}

export function ImagePreview({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  on_resize_start,
  embedded,
}: PreviewFrameProps) {
  const [is_loaded, setIsLoaded] = useState(false);
  const [has_error, setHasError] = useState(false);
  const file_action_copy = get_workspace_file_external_action_copy(file_name);
  const preview_url = get_workspace_file_preview_url(agent_id, path);

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          aria_label="调整编辑器宽度"
          class_name="flex"
          on_mouse_down={on_resize_start}
        />
      ) : null}

      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <>
            <span className="flex items-center gap-1">
              <FileImage className="h-3 w-3" />
              图片预览
            </span>
            {has_error ? (
              <span className="flex items-center gap-1 text-destructive">
                <EyeOff className="h-3 w-3" />
                加载失败
              </span>
            ) : is_loaded ? (
              <span className="flex items-center gap-1 text-(--success)">
                <Eye className="h-3 w-3" />
                已加载
              </span>
            ) : (
              <span className="flex items-center gap-1">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                加载中
              </span>
            )}
          </>
        )}
        title={file_name}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)] p-6">
        {has_error ? (
          <div className="m-auto text-center">
            <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
            <p className="mt-4 text-sm font-medium text-(--text-strong)">图片加载失败</p>
            <p className="mt-2 text-xs text-(--text-soft)">
              请尝试{file_action_copy.label}文件
            </p>
          </div>
        ) : (
          <img
            className="max-h-full max-w-full rounded-lg object-contain"
            src={preview_url}
            alt={file_name}
            onLoad={() => setIsLoaded(true)}
            onError={() => {
              setIsLoaded(true);
              setHasError(true);
            }}
          />
        )}
      </div>
    </>
  );
}

export function BinaryFilePlaceholder({
  agent_id,
  path,
  file_name,
  is_preview_focused,
  on_toggle_preview_focus,
  embedded,
}: BinaryFilePlaceholderProps) {
  const file_action_copy = get_workspace_file_external_action_copy(file_name);
  const action_description = file_action_copy.mode === "reveal"
    ? "在文件夹中显示此文件"
    : "获取此文件";
  return (
    <>
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agent_id={agent_id} file_name={file_name} path={path} />
            <WorkspaceFilePreviewFocusButton
              is_preview_focused={is_preview_focused}
              on_toggle_preview_focus={on_toggle_preview_focus}
            />
          </>
        )}
        embedded={embedded}
        meta={(
          <span className="flex items-center gap-1">
            <FileWarning className="h-3 w-3" />
            此文件类型不支持预览
          </span>
        )}
        title={file_name}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)] p-8">
        <div className="m-auto max-w-xs text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-2xl border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
            <FileWarning className="h-8 w-8 text-(--icon-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-strong)">不支持预览此文件</p>
          <p className="mt-2 text-xs leading-5 text-(--text-soft)">
            当前预览仅支持文本、PDF、图片、xlsx、docx 和 pptx 文件。您可以点击上方"{file_action_copy.label}"按钮{action_description}。
          </p>
        </div>
      </div>
    </>
  );
}
