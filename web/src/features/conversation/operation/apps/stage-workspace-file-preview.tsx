/**
 * INPUT: An explicit workspace file-open intent from an Operation Stage event.
 * OUTPUT: The same live, interactive preview implementation used by the workspace editor.
 * POS: Stage document-app adapter; it must not maintain a second fake document renderer.
 */
import { FileWarning } from "lucide-react";

import { getWorkspaceFilePreviewKind } from "@/features/conversation/shared/editor/workspace-file-preview-kind";
import { WorkspaceFilePreviewRouter } from "@/features/conversation/shared/editor/workspace-file-preview-router";

export function StageWorkspaceFilePreview({
  agentId,
  initialContent,
  path,
}: {
  agentId: string;
  initialContent?: string | null;
  path: string;
}) {
  const normalized_path = normalize_workspace_preview_path(path);
  if (!agentId.trim() || !normalized_path) {
    return (
      <div className="grid h-full min-h-[240px] place-items-center bg-(--surface-panel-subtle-background) p-8 text-center">
        <div className="max-w-sm">
          <FileWarning className="mx-auto h-8 w-8 text-(--icon-muted)" />
          <p className="mt-3 text-[12px] font-semibold text-(--text-strong)">无法打开工作区外的文件</p>
          <p className="mt-1 text-[11px] leading-5 text-(--text-soft)">{path}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-[240px] min-w-0 flex-col overflow-hidden bg-(--surface-panel-background)">
      <WorkspaceFilePreviewRouter
        agentId={agentId}
        fileName={normalized_path.split("/").at(-1) ?? normalized_path}
        fileType={getWorkspaceFilePreviewKind(normalized_path)}
        initialContent={initialContent}
        isPreviewFocused
        onTogglePreviewFocus={() => undefined}
        path={normalized_path}
        showFocusControl={false}
      />
    </div>
  );
}

function normalize_workspace_preview_path(path: string): string | null {
  const normalized = path.trim().replace(/\\/g, "/").replace(/^\.\//, "");
  if (!normalized || normalized.startsWith("/") || normalized.split("/").includes("..")) {
    return null;
  }
  return normalized;
}
