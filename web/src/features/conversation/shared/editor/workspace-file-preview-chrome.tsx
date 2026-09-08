// INPUT: Workspace 文件层级、预览状态、文件动作、owner 代次与可选标题栏 Portal。
// OUTPUT: 共享文件 chrome；外部操作反馈只属于当前文件/owner 的最近一次操作，文案随语言更新。
// POS: Workspace 文件预览外壳；文件动作生命周期归公共领域 Hook，不读取文件内容或拥有全站导航视觉。
"use client";

import {
  createContext,
  type ReactNode,
  useContext,
} from "react";
import { createPortal } from "react-dom";
import {
  Download,
  FolderOpen,
  Maximize2,
  Minimize2,
} from "lucide-react";

import { useWorkspaceFileExternalAction } from "@/hooks/agent/use-workspace-file-external-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiBreadcrumb } from "@/shared/ui/navigation/breadcrumb";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import {
  WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
  WORKSPACE_PANEL_HEADER_ICON_CLASS,
  WORKSPACE_PANEL_HEADER_PADDING_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";

interface WorkspaceFilePreviewHeaderContextValue {
  headerPortalTarget?: HTMLElement | null;
  leading?: ReactNode;
  locationSegments: readonly string[];
}

const WorkspaceFilePreviewHeaderContext =
  createContext<WorkspaceFilePreviewHeaderContextValue>({ locationSegments: [] });

export function WorkspaceFilePreviewHeaderProvider({
  children,
  headerPortalTarget,
  leading,
  locationSegments,
}: {
  children: ReactNode;
  headerPortalTarget?: HTMLElement | null;
  leading?: ReactNode;
  locationSegments: readonly string[];
}) {
  return (
    <WorkspaceFilePreviewHeaderContext.Provider
      value={{ headerPortalTarget, leading, locationSegments }}
    >
      {children}
    </WorkspaceFilePreviewHeaderContext.Provider>
  );
}

export function WorkspaceFilePreviewHeader({
  actions,
  meta,
  title,
}: {
  actions: ReactNode;
  meta?: ReactNode;
  title: string;
}) {
  const { t } = useI18n();
  const { headerPortalTarget, leading, locationSegments } = useContext(
    WorkspaceFilePreviewHeaderContext,
  );
  const header = (
    <header
      className={cn(
        "flex min-w-0 shrink-0 items-center gap-3 overflow-hidden border-b divider-subtle",
        WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
        WORKSPACE_PANEL_HEADER_PADDING_CLASS,
        headerPortalTarget && "h-full min-h-0 border-b-0 px-0",
      )}
    >
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <UiBreadcrumb
          ariaLabel={t("common.location_aria")}
          className="min-w-0 flex-1"
          density="compact"
          items={[
            ...locationSegments.map((segment, index) => ({
              id: `location-${index}`,
              label: segment,
              title: segment,
            })),
            { id: "file", label: title, title },
          ]}
          leading={leading}
        />
        {meta ? (
          <div className={cn(
            "hidden min-w-0 shrink items-center gap-2 overflow-hidden whitespace-nowrap sm:flex",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {meta}
          </div>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-0.5">{actions}</div>
    </header>
  );
  if (headerPortalTarget === null) {
    return null;
  }
  return headerPortalTarget ? createPortal(header, headerPortalTarget) : header;
}

export function WorkspaceFileDownloadButton({
  agentId,
  path,
  fileName,
}: {
  agentId: string;
  path: string;
  fileName: string;
}) {
  const { copy: fileActionCopy, disabled, failure, onAction } = useWorkspaceFileExternalAction({ agentId, path, fileName });

  return (
    <>
      <UiIconButton
        aria-label={fileActionCopy.ariaLabel}
        disabled={disabled}
        onClick={onAction}
        size="sm"
        tooltip={fileActionCopy.title}
        variant="ghost"
      >
        {fileActionCopy.mode === "reveal" ? (
          <FolderOpen className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
        ) : (
          <Download className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
        )}
      </UiIconButton>
      <FeedbackBannerViewport item={failure} />
    </>
  );
}

export function WorkspaceFileToolbarButton({
  children,
  disabled = false,
  onClick,
  title,
}: {
  children: ReactNode;
  disabled?: boolean;
  onClick: () => void;
  title: string;
}) {
  return (
    <UiIconButton
      aria-label={title}
      disabled={disabled}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      size="sm"
      tooltip={title}
      variant="ghost"
    >
      {children}
    </UiIconButton>
  );
}

export function WorkspaceFilePreviewFocusButton({
  isPreviewFocused,
  onTogglePreviewFocus,
}: {
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
}) {
  const { t } = useI18n();
  return (
    <WorkspaceFileToolbarButton
      onClick={onTogglePreviewFocus}
      title={t(isPreviewFocused
        ? "workspace_file.show_file_list"
        : "workspace_file.focus_preview")}
    >
      {isPreviewFocused ? (
        <Minimize2 className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
      ) : (
        <Maximize2 className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
      )}
    </WorkspaceFileToolbarButton>
  );
}
