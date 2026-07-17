/**
 * INPUT: An explicit workspace file-open intent from an Operation Stage event.
 * OUTPUT: The same live, interactive preview implementation used by the workspace editor.
 * POS: Stage document-app adapter; it must not maintain a second fake document renderer.
 */
import { FileWarning } from "lucide-react";
import { useMemo } from "react";

import { getWorkspaceFilePreviewKind } from "@/features/conversation/shared/editor/workspace-file-preview-kind";
import { WorkspaceFilePreviewRouter } from "@/features/conversation/shared/editor/workspace-file-preview-router";
import { useAgentStore } from "@/store/agent";
import { useWorkspaceFilesStore } from "@/store/workspace-files";
import { useWorkspaceLiveStore } from "@/store/workspace-live";

import { resolveOperationWorkspaceFilePath } from "../operation-workspace-file-path";

export function StageWorkspaceFilePreview({
  agentId,
  initialContent,
  path,
}: {
  agentId: string;
  initialContent?: string | null;
  path: string;
}) {
  const workspace_path = useAgentStore((state) => (
    state.agents.find((agent) => agent.agent_id === agentId)?.workspace_path ?? null
  ));
  const workspace_entries = useWorkspaceFilesStore((state) => state.files_by_agent[agentId]);
  const known_paths = useMemo(
    () => workspace_entries?.map((entry) => entry.path) ?? [],
    [workspace_entries],
  );
  const normalized_path = resolveOperationWorkspaceFilePath({
    knownPaths: known_paths,
    path,
    workspacePath: workspace_path,
  });
  const live_state = useWorkspaceLiveStore((state) => (
    normalized_path ? state.file_states[`${agentId}:${normalized_path}`] : undefined
  ));
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

  const file_type = getWorkspaceFilePreviewKind(normalized_path);
  const is_text_renderer = file_type === "html"
    || file_type === "markdown"
    || file_type === "mermaid"
    || file_type === "text";
  const settled_version = live_state?.status === "updated" ? live_state.version : 0;
  const renderer_key = is_text_renderer
    ? normalized_path
    : `${normalized_path}:${settled_version}`;
  const resolved_initial_content = typeof live_state?.live_content === "string"
    ? live_state.live_content
    : initialContent;

  return (
    <div className="flex h-full min-h-[240px] min-w-0 flex-col overflow-hidden bg-(--surface-panel-background)">
      <WorkspaceFilePreviewRouter
        agentId={agentId}
        fileName={normalized_path.split("/").at(-1) ?? normalized_path}
        fileType={file_type}
        initialContent={resolved_initial_content}
        isPreviewFocused
        key={renderer_key}
        onTogglePreviewFocus={() => undefined}
        path={normalized_path}
        showFocusControl={false}
      />
    </div>
  );
}
