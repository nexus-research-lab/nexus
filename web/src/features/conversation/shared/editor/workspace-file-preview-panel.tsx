// INPUT: 当前 exact Agent、文件路径和预览布局状态。
// OUTPUT: 路径切换时重新建立的文件预览与编辑器实例。
// POS: Workspace 文件预览 scope 边界；旧文件草稿不得复用于新路径。
import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

import { WorkspaceFilePreviewHeaderProvider } from "./workspace-file-preview-chrome";
import { getWorkspaceFilePreviewKind } from "./workspace-file-preview-kind";
import { WorkspaceFilePreviewRouter } from "./workspace-file-preview-router";

interface WorkspaceFilePreviewPanelProps {
  agentId: string;
  className?: string;
  headerLeading?: ReactNode;
  headerLocationLabel: string;
  headerPortalTarget?: HTMLElement | null;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string | null;
}

function WorkspaceFilePreviewEmptyState() {
  const { t } = useI18n();
  return (
    <div className="flex h-full flex-1 items-center justify-center px-8 text-center">
      <div className="max-w-sm">
        <p className="text-sm font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          {t("room.workspace_preview_title")}
        </p>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          {t("room.workspace_preview_empty_description")}
        </p>
      </div>
    </div>
  );
}

/** 路径是预览打开态的唯一来源，面板不维护可与路径冲突的镜像状态。 */
export function WorkspaceFilePreviewPanel({
  agentId,
  className,
  headerLeading,
  headerLocationLabel,
  headerPortalTarget,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewPanelProps) {
  if (!path) {
    return (
      <section className={cn("relative flex min-h-0 min-w-0 flex-col overflow-hidden", className)}>
        <WorkspaceFilePreviewEmptyState />
      </section>
    );
  }

  return (
    <section className={cn("relative flex min-h-0 min-w-0 flex-col overflow-hidden", className)}>
      <WorkspaceFilePreviewHeaderProvider
        headerPortalTarget={headerPortalTarget}
        leading={headerLeading}
        locationLabel={headerLocationLabel}
      >
        <WorkspaceFilePreviewRouter
          agentId={agentId}
          fileName={path.split("/").at(-1) ?? ""}
          fileType={getWorkspaceFilePreviewKind(path)}
          isPreviewFocused={isPreviewFocused}
          key={`${agentId}\u0000${path}`}
          onTogglePreviewFocus={onTogglePreviewFocus}
          path={path}
        />
      </WorkspaceFilePreviewHeaderProvider>
    </section>
  );
}
