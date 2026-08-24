import type { ReactNode } from "react";

import { ExecutionWorkGraphSurface } from "@/features/conversation/shared/execution/execution-workgraph-surface";
import { buildExecutionAgentDirectory } from "@/features/conversation/shared/execution/execution-process-model";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { useI18n } from "@/shared/i18n/i18n-context";
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
import { RoomSubagentTaskSurface } from "../room-subagent-task-surface";
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
  composerDraftScopeKey: string | null;
  currentAgent: Agent;
  executionResource: ExecutionResource;
  executionTaskRuns: ConversationTaskRun[];
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
  subagentRequest: {
    hostAgentId: string | null;
    key: number;
    toolUseId: string | null;
  };
  subagentTaskSource: SubagentTaskSource | null;
}

export function RoomSurfaceAuxiliaryPanel({
  aboutRequest,
  activeSurfaceTab,
  activeWorkspacePath,
  conversationId,
  composerDraftScopeKey,
  currentAgent,
  executionResource,
  executionTaskRuns,
  sidePanelWidthPercent,
  isDm,
  onClose,
  onOpenWorkspaceFile,
  onSaveAgentOptions,
  onStartSidePanelResize,
  onValidateAgentName,
  roomId,
  roomMembers,
  subagentRequest,
  subagentTaskSource,
}: RoomSurfaceAuxiliaryPanelProps) {
  const { t } = useI18n();
  const executionAgents = [
    ...roomMembers.filter((agent) => agent.agent_id !== currentAgent.agent_id),
    currentAgent,
  ];
  const executionDirectory = buildExecutionAgentDirectory(executionAgents);
  const persistentPanels: Array<{
    content: ReactNode;
    key: "workgraph" | "workspace" | "about";
  }> = [
    {
      key: "workgraph",
      content: (
        <ExecutionWorkGraphSurface
          agents={executionAgents}
          directory={executionDirectory}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          resource={executionResource}
          taskRuns={executionTaskRuns}
        />
      ),
    },
    {
      key: "workspace",
      content: (
        <RoomWorkspaceView
          activeWorkspacePath={activeWorkspacePath}
          agentId={currentAgent.agent_id}
          composerDraftScopeKey={composerDraftScopeKey}
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
    <>
      <PanelResizeHandle
        ariaLabel={t("room.resize_auxiliary_panel")}
        onResizeStart={onStartSidePanelResize}
        variant="gutter"
      />

      <section
        className="nexus-room-surface-side-panel relative flex min-h-0 min-w-0 shrink-0 flex-col overflow-hidden"
        style={{
          width: `${sidePanelWidthPercent}%`,
          ...AUXILIARY_PANEL_WIDTH_LIMITS,
        }}
      >
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
            <RoomSubagentTaskSurface
              currentAgentId={currentAgent.agent_id}
              onClose={onClose}
              onOpenWorkspaceFile={onOpenWorkspaceFile}
              requestKey={subagentRequest.key}
              requestedHostAgentId={subagentRequest.hostAgentId}
              requestedTaskToolUseId={subagentRequest.toolUseId}
              roomMembers={roomMembers}
              source={subagentTaskSource}
            />
          </div>
        ) : null}
      </section>
    </>
  );
}
