"use client";

import { cn } from "@/shared/ui/class-name";

import { getWorkspaceFilePreviewKind } from "./workspace-file-preview-kind";
import { WorkspaceFilePreviewRouter } from "./workspace-file-preview-router";

interface EditorPanelProps {
  agentId: string;
  className?: string;
  embedded?: boolean;
  isOpen: boolean;
  isPreviewFocused?: boolean;
  onResizeStart: () => void;
  onTogglePreviewFocus?: () => void;
  path: string | null;
  widthPercent: number;
}

function EmbeddedEditorEmptyState() {
  return (
    <div className="flex h-full flex-1 items-center justify-center px-8 text-center">
      <div className="max-w-sm">
        <p className="text-sm font-semibold uppercase tracking-[0.18em] text-muted-foreground">
          Workspace Preview
        </p>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          从左侧选择一个文件，这里会显示对应内容。模型写入时，也会在这里实时同步。
        </p>
      </div>
    </div>
  );
}

export function EditorPanel({
  agentId,
  className,
  embedded = false,
  isOpen,
  isPreviewFocused = false,
  onResizeStart,
  onTogglePreviewFocus,
  path,
  widthPercent,
}: EditorPanelProps) {
  if (!embedded && !isOpen) {
    return null;
  }
  const fileName = path?.split("/").at(-1) || "";
  const fileType = path
    ? getWorkspaceFilePreviewKind(path)
    : "unknown";

  return (
    <section
      className={cn(
        "relative flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden transition-[width,opacity,transform,border-color] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]",
        embedded
          ? "border-0 bg-transparent shadow-none"
          : "border-l divider-subtle bg-transparent shadow-none",
        isOpen
          ? "translate-x-0 opacity-100"
          : "pointer-events-none -translate-x-3 opacity-0",
        className,
      )}
      style={embedded
        ? { width: "100%" }
        : { width: isOpen ? `${widthPercent}%` : "0px" }}
    >
      {embedded && (!isOpen || !path) ? <EmbeddedEditorEmptyState /> : null}
      {isOpen && path ? (
        <WorkspaceFilePreviewRouter
          agentId={agentId}
          embedded={embedded}
          fileName={fileName}
          fileType={fileType}
          isPreviewFocused={isPreviewFocused}
          onResizeStart={onResizeStart}
          onTogglePreviewFocus={onTogglePreviewFocus}
          path={path}
        />
      ) : null}
    </section>
  );
}
