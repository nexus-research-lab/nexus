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
          onTogglePreviewFocus={onTogglePreviewFocus}
          path={path}
        />
      </WorkspaceFilePreviewHeaderProvider>
    </section>
  );
}
