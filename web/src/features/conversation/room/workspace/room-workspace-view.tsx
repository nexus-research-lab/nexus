"use client";

import { useRef } from "react";

import { WorkspaceFilePreviewPanel } from "@/features/conversation/shared/editor/workspace-file-preview-panel";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
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
  roomMembers: Agent[];
  onOpenWorkspaceFile: (path: string | null) => void;
}

export function RoomWorkspaceView({
  activeWorkspacePath,
  agentId,
  isDm,
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
        maxWidthClassName="max-w-none"
        title={t("room.workspace_title")}
      >
        <div
          ref={fileListLayout.panelRef}
          className={cn(
            "flex h-full min-h-0 min-w-0 flex-1",
            fileListLayout.isResizing && "cursor-col-resize select-none",
          )}
        >
          <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
            <WorkspaceFilePreviewPanel
              agentId={controller.agent.viewAgentId}
              className="h-full w-full"
              headerLeading={agentSwitcher}
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={togglePreviewFocus}
              path={activeWorkspacePath}
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
