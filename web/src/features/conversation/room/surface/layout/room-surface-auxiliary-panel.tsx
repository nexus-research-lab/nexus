import type { ReactNode } from "react";

import { SubagentTaskSurface } from "@/features/conversation/shared/subagent/subagent-task-surface";
import { cn } from "@/shared/ui/class-name";
import { PanelResizeHandle } from "@/shared/ui/layout/panel-resize-handle";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomAgentAboutSurface } from "../room-agent-about-surface";
import { RoomWorkspaceView } from "../../workspace/room-workspace-view";
import type { RoomAgentAboutRequest } from "./room-surface-layout-types";

const AUXILIARY_PANEL_WIDTH_LIMITS = {
  minWidth: "min(520px, 46vw)",
  maxWidth: "min(860px, 54vw)",
};

interface RoomSurfaceAuxiliaryPanelProps {
  aboutRequest: RoomAgentAboutRequest;
  activeSurfaceTab: RoomSurfaceTabKey;
  activeWorkspacePath: string | null;
  conversationId: string | null;
  currentAgent: Agent;
  sidePanelWidthPercent: number;
  isDm: boolean;
  onClose: () => void;
  onOpenWorkspaceFile: (
    path: string | null,
    workspaceAgentId?: string | null,
  ) => void;
  onSaveAgentOptions: (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
  onStartSidePanelResize: () => void;
  onValidateAgentName: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
  roomId: string | null;
  roomMembers: Agent[];
  subagentTaskSource: SubagentTaskSource | null;
}

export function RoomSurfaceAuxiliaryPanel({
  aboutRequest,
  activeSurfaceTab,
  activeWorkspacePath,
  conversationId,
  currentAgent,
  sidePanelWidthPercent,
  isDm,
  onClose,
  onOpenWorkspaceFile,
  onSaveAgentOptions,
  onStartSidePanelResize,
  onValidateAgentName,
  roomId,
  roomMembers,
  subagentTaskSource,
}: RoomSurfaceAuxiliaryPanelProps) {
  const persistentPanels: Array<{
    content: ReactNode;
    key: "workspace" | "about";
  }> = [
    {
      key: "workspace",
      content: (
        <RoomWorkspaceView
          activeWorkspacePath={activeWorkspacePath}
          agentId={currentAgent.agent_id}
          isDm={isDm}
          roomMembers={roomMembers}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
        />
      ),
    },
    {
      key: "about",
      content: (
        <RoomAgentAboutSurface
          agent={currentAgent}
          conversationId={conversationId}
          roomId={roomId}
          roomMembers={roomMembers}
          isVisible={activeSurfaceTab === "about"}
          requestedAgentId={aboutRequest.agent_id}
          requestedTab={aboutRequest.tab}
          requestKey={aboutRequest.key}
          onSaveAgentOptions={onSaveAgentOptions}
          onValidateAgentName={onValidateAgentName}
        />
      ),
    },
  ];

  return (
    <section
      className="relative ml-2 flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden border-l divider-subtle bg-transparent shadow-none"
      style={{
        width: `${sidePanelWidthPercent}%`,
        ...AUXILIARY_PANEL_WIDTH_LIMITS,
      }}
    >
      <PanelResizeHandle
        ariaLabel="调整右侧面板宽度"
        onResizeStart={onStartSidePanelResize}
      />

      {persistentPanels.map((panel) => (
        <div
          key={panel.key}
          className={cn(
            "flex h-full min-h-0 min-w-0 flex-1 flex-col",
            activeSurfaceTab !== panel.key && "hidden",
          )}
        >
          {panel.content}
        </div>
      ))}

      {activeSurfaceTab === "subagents" && subagentTaskSource ? (
        <div className="flex h-full min-h-0 min-w-0 flex-1 flex-col">
          <SubagentTaskSurface
            onClose={onClose}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            source={subagentTaskSource}
          />
        </div>
      ) : null}
    </section>
  );
}
