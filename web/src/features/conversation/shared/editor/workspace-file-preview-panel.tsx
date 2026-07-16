import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

import { WorkspaceFilePreviewHeaderProvider } from "./workspace-file-preview-chrome";
import { getWorkspaceFilePreviewKind } from "./workspace-file-preview-kind";
import { WorkspaceFilePreviewRouter } from "./workspace-file-preview-router";

interface WorkspaceFilePreviewPanelProps {
  agentId: string;
  className?: string;
  headerLeading?: ReactNode;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string | null;
}

function WorkspaceFilePreviewEmptyState({ leading }: { leading?: ReactNode }) {
  return (
    <>
      {leading ? (
        <div className="flex h-7 shrink-0 items-start border-b divider-subtle px-3">
          {leading}
        </div>
      ) : null}
      <div className="flex h-full flex-1 items-center justify-center px-8 text-center">
        <div className="max-w-sm">
          <p className="text-sm font-semibold uppercase tracking-[0.18em] text-muted-foreground">
            Workspace Preview
          </p>
          <p className="mt-3 text-sm leading-6 text-muted-foreground">
            从文件列表选择一个文件，这里会显示对应内容。模型写入时，也会在这里实时同步。
          </p>
        </div>
      </div>
    </>
  );
}

/** 路径是预览打开态的唯一来源，面板不维护可与路径冲突的镜像状态。 */
export function WorkspaceFilePreviewPanel({
  agentId,
  className,
  headerLeading,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewPanelProps) {
  if (!path) {
    return (
      <section className={cn("relative flex min-h-0 min-w-0 flex-col overflow-hidden", className)}>
        <WorkspaceFilePreviewEmptyState leading={headerLeading} />
      </section>
    );
  }

  return (
    <section className={cn("relative flex min-h-0 min-w-0 flex-col overflow-hidden", className)}>
      <WorkspaceFilePreviewHeaderProvider leading={headerLeading}>
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
