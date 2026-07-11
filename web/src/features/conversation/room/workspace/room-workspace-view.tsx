"use client";

import { useRef } from "react";

import { EditorPanel } from "@/features/conversation/shared/editor/editor-panel";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { Agent } from "@/types/agent/agent";

import { RoomAgentSwitcher } from "../surface/room-agent-switcher";
import { useRoomWorkspaceController } from "./controller/use-room-workspace-controller";
import { useWorkspaceFileListLayout } from "./view/use-workspace-file-list-layout";
import { WorkspaceDialogs } from "./view/workspace-dialogs";
import { WorkspaceFileBrowser } from "./view/workspace-file-browser";

interface RoomWorkspaceViewProps {
  activeWorkspacePath: string | null;
  agentId: string;
  isDm: boolean;
  isEditorOpen: boolean;
  roomMembers: Agent[];
  onOpenWorkspaceFile: (path: string | null) => void;
}

export function RoomWorkspaceView({
  activeWorkspacePath,
  agentId,
  isDm,
  isEditorOpen,
  roomMembers,
  onOpenWorkspaceFile,
}: RoomWorkspaceViewProps) {
  const {t} = useI18n();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const fileListLayout = useWorkspaceFileListLayout();
  const [isPreviewFocused, setIsPreviewFocused] = useResettableState(
    false,
    activeWorkspacePath ? "has-path" : "no-path",
  );
  const controller = useRoomWorkspaceController({
    activeWorkspacePath,
    agentId,
    isDm,
    onOpenWorkspaceFile,
    fileInputRef,
  });

  const togglePreviewFocus = () => {
    setIsPreviewFocused((current) => !current);
    fileListLayout.stopResizing();
  };
  const agentSwitcher = !isDm && roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      members={roomMembers}
      onSelect={controller.agent.onSelect}
      selectedId={controller.agent.selectedId}
    />
  ) : null;

  return (
    <>
      <input
        ref={fileInputRef}
        aria-label="上传工作区文件"
        className="hidden"
        multiple
        onChange={controller.fileInput.onChange}
        type="file"
      />

      <WorkspaceSurfaceView
        bodyClassName="px-2 pt-1 pb-0 sm:px-2 xl:px-4"
        bodyScrollable={false}
        contentClassName="flex h-full min-h-0 min-w-0 gap-4"
        eyebrow={t("room.workspace")}
        maxWidthClassName="max-w-none"
        showEyebrow={false}
        showTitle={false}
        title={t("room.workspace_title")}
        titleTrailing={agentSwitcher}
      >
        <div
          ref={fileListLayout.panelRef}
          className={cn(
            "flex h-full min-h-0 min-w-0 flex-1",
            fileListLayout.isResizing && "cursor-col-resize select-none",
          )}
        >
          <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
            <EditorPanel
              agentId={controller.agent.viewAgentId}
              className="h-full w-full"
              embedded
              isOpen={isEditorOpen}
              isPreviewFocused={isPreviewFocused}
              onResizeStart={() => undefined}
              onTogglePreviewFocus={activeWorkspacePath ? togglePreviewFocus : undefined}
              path={activeWorkspacePath}
              widthPercent={100}
            />
          </div>

          {!isPreviewFocused ? (
            <WorkspaceFileBrowser
              activePath={activeWorkspacePath}
              controller={controller.browser}
              onResizeStart={fileListLayout.startResizing}
              width={fileListLayout.width}
            />
          ) : null}
        </div>
      </WorkspaceSurfaceView>

      <WorkspaceDialogs controller={controller.dialogs} />
    </>
  );
}
