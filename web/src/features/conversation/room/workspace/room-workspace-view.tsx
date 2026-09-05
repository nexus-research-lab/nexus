// INPUT: Room/DM 的 Workspace 选择、当前相对路径、成员身份与面板布局状态。
// OUTPUT: 文件目录、预览、切换器和弹窗组成的响应式 Workspace 工作面。
// POS: Conversation Workspace 组合层；不拥有文件事务或公共 Breadcrumb 视觉。
"use client";

import { useRef, useState } from "react";

import { WorkspaceFilePreviewPanel } from "@/features/conversation/shared/editor/workspace-file-preview-panel";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import {
  WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
  WORKSPACE_PANEL_HEADER_PADDING_CLASS,
} from "@/shared/ui/workspace/surface/workspace-header-layout";
import { WorkspaceSurfaceView } from "@/shared/ui/workspace/surface/workspace-surface-view";
import type { Agent } from "@/types/agent/agent";

import { RoomAgentSwitcher } from "../surface/room-agent-switcher";
import { useRoomWorkspaceController } from "./controller/use-room-workspace-controller";
import {
  getWorkspaceFileLocationSegments,
  getWorkspaceRootLabel,
} from "./controller/workspace-path-model";
import { useWorkspaceFileListLayout } from "./view/use-workspace-file-list-layout";
import { WorkspaceDialogs } from "./view/workspace-dialogs";
import {
  WorkspaceDirectoryToolbar,
  WorkspaceFileBrowser,
} from "./view/workspace-file-browser";

interface RoomWorkspaceViewProps {
  activeWorkspacePath: string | null;
  agentId: string;
  composerDraftScopeKey: string | null;
  compact?: boolean;
  isDm: boolean;
  roomMembers: Agent[];
  onOpenWorkspaceFile: (path: string | null) => void;
}

export function RoomWorkspaceView({
  activeWorkspacePath,
  agentId,
  composerDraftScopeKey,
  compact = false,
  isDm,
  roomMembers,
  onOpenWorkspaceFile,
}: RoomWorkspaceViewProps) {
  const {t} = useI18n();
  const isStacked = useMediaQuery("(max-width: 639px)") && compact;
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [previewHeaderTarget, setPreviewHeaderTarget] =
    useState<HTMLDivElement | null>(null);
  const fileListLayout = useWorkspaceFileListLayout();
  const [isPreviewFocused, setIsPreviewFocused] = useResettableState(
    false,
    activeWorkspacePath ? "has-path" : "no-path",
  );
  const controller = useRoomWorkspaceController({
    activeWorkspacePath,
    agentId,
    composerDraftScopeKey,
    isDm,
    onOpenWorkspaceFile,
    fileInputRef,
    roomMembers,
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
      variant="panel"
    />
  ) : null;
  const viewAgent = roomMembers.find(
    (member) => member.agent_id === controller.agent.viewAgentId,
  );
  const workspaceRootLabel = getWorkspaceRootLabel(
    viewAgent?.display_name,
    viewAgent?.name,
    t("room.workspace_title"),
  );
  const headerLocationSegments = activeWorkspacePath
    ? getWorkspaceFileLocationSegments(activeWorkspacePath, workspaceRootLabel)
    : [workspaceRootLabel];

  return (
    <>
      <input
        ref={fileInputRef}
        aria-label={t("room.workspace_action_upload")}
        className="hidden"
        multiple
        onChange={controller.fileInput.onChange}
        type="file"
      />

      <WorkspaceSurfaceView
        bodyClassName="px-0 py-0"
        bodyScrollable={false}
        contentClassName="flex h-full min-h-0 min-w-0 gap-4"
        maxWidthClassName="max-w-none"
        title={t("room.workspace_title")}
      >
        <div
          ref={fileListLayout.panelRef}
          className={cn(
            "flex h-full min-h-0 min-w-0 flex-1 flex-col",
            fileListLayout.isResizing && "cursor-col-resize select-none",
          )}
        >
          <div className={cn(
            "flex min-w-0 shrink-0 items-center border-b divider-subtle",
            WORKSPACE_PANEL_HEADER_HEIGHT_CLASS,
            WORKSPACE_PANEL_HEADER_PADDING_CLASS,
          )}>
            <div ref={setPreviewHeaderTarget} className="h-full min-w-0 flex-1">
              {!activeWorkspacePath ? (
                <div className="flex h-full min-w-0 items-center">
                  {agentSwitcher ?? (
                    <span className="truncate text-xs font-normal text-(--text-soft)">
                      {workspaceRootLabel}
                    </span>
                  )}
                </div>
              ) : null}
            </div>
            {!isPreviewFocused ? (
              <WorkspaceDirectoryToolbar controller={controller.browser} />
            ) : null}
          </div>

          <div
            className={cn(
              "flex min-h-0 min-w-0 flex-1 px-2 xl:px-4",
              isStacked && "flex-col-reverse gap-3",
            )}
          >
            <div className="min-h-0 min-w-0 flex-1 overflow-hidden">
              <WorkspaceFilePreviewPanel
                agentId={controller.agent.viewAgentId}
                className="h-full w-full"
                headerLeading={agentSwitcher}
                headerLocationSegments={headerLocationSegments}
                headerPortalTarget={previewHeaderTarget}
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
                stacked={isStacked}
                width={fileListLayout.width}
              />
            ) : null}
          </div>
        </div>
      </WorkspaceSurfaceView>

      <WorkspaceDialogs controller={controller.dialogs} />
      <FeedbackBannerViewport item={controller.feedback} />
    </>
  );
}
