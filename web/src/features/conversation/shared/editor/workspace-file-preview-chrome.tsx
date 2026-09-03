// INPUT: Workspace 文件层级、预览状态、文件动作与可选标题栏 Portal。
// OUTPUT: 复用 UiBreadcrumb 的单行文件 chrome，以及统一下载、聚焦和编辑动作。
// POS: Workspace 文件预览外壳；不读取文件内容，也不拥有全站导航视觉。
"use client";

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useState,
} from "react";
import { createPortal } from "react-dom";
import {
  Download,
  FolderOpen,
  Maximize2,
  Minimize2,
} from "lucide-react";

import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiBreadcrumb } from "@/shared/ui/navigation/breadcrumb";
import {
  WORKSPACE_PANEL_HEADER_BUTTON_CLASS,
  WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
  WORKSPACE_PANEL_HEADER_ICON_CLASS,
  WORKSPACE_PANEL_HEADER_PADDING_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";

const WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME = cn(
  "inline-flex items-center justify-center rounded-[6px] text-(--text-default) transition-colors",
  WORKSPACE_PANEL_HEADER_BUTTON_CLASS,
  "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]",
  "disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) disabled:hover:bg-transparent disabled:hover:text-(--text-default)",
);

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
          <div className="hidden min-w-0 shrink items-center gap-2 overflow-hidden whitespace-nowrap text-2xs text-(--text-soft) sm:flex">
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
  const { t } = useI18n();
  const fileActionCopy = getWorkspaceFileExternalActionCopy(t, fileName);
  const [failure, setFailure] = useState<FeedbackBannerProps | null>(null);
  const handleExternalAction = useCallback(() => {
    setFailure(null);
    void downloadWorkspaceFileApi(agentId, path, fileName).catch((error) => {
      console.error(`[WorkspaceFileDownloadButton] ${fileActionCopy.label} workspace 文件失败:`, error);
      setFailure({
        impact: t("workspace_file.external_action_failed_impact"),
        nextStep: t("workspace_file.external_action_failed_next_step"),
        onDismiss: () => setFailure(null),
        title: t("workspace_file.external_action_failed"),
        tone: "error",
        urgency: "polite",
      });
    });
  }, [agentId, fileActionCopy.label, fileName, path, t]);

  return (
    <>
      <button
        aria-label={fileActionCopy.ariaLabel}
        className={WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME}
        onClick={handleExternalAction}
        title={fileActionCopy.title}
        type="button"
      >
        {fileActionCopy.mode === "reveal" ? (
          <FolderOpen className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
        ) : (
          <Download className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
        )}
      </button>
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
    <button
      aria-label={title}
      className={WORKSPACE_FILE_TOOLBAR_BUTTON_CLASS_NAME}
      disabled={disabled}
      onMouseDown={(event) => event.preventDefault()}
      onClick={onClick}
      title={title}
      type="button"
    >
      {children}
    </button>
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
