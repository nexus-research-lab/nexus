// INPUT: Room 辅助页内容、共享右栏百分比与页面/布局命令。
// OUTPUT: 按需首次挂载、右栏打开期间保留已访问内容的辅助工作面及具名键盘分隔条。
// POS: 右栏内容装配；宽度限制与交互分别归布局模型、共享分隔条。

import { useSyncExternalStore } from "react";

import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { captureAuthOwnerScopeGeneration, subscribeAuthOwnerScopeGeneration } from "@/shared/auth/auth-owner-generation";
import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomAuxiliaryTabContent } from "../room-auxiliary-tab-content";
import { RoomSidePanelContainer } from "./room-side-panel-container";
import type { RoomAgentAboutRequest } from "./room-surface-layout-types";

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
  onSidePanelWidthChange: (percent: number) => void;
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
  onSidePanelWidthChange,
  onValidateAgentName,
  roomId,
  roomMembers,
  subagentRequest,
  subagentTaskSource,
}: RoomSurfaceAuxiliaryPanelProps) {
  const { t } = useI18n();
  const ownerGeneration = useSyncExternalStore(
    subscribeAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration, captureAuthOwnerScopeGeneration,
  );
  const visitScope = JSON.stringify([ownerGeneration, roomId, isDm, isDm ? currentAgent.agent_id : null]);
  const [visitedTabs, setVisitedTabs] = useResettableState<RoomSurfaceTabKey[]>([activeSurfaceTab], visitScope);
  if (!visitedTabs.includes(activeSurfaceTab)) {
    setVisitedTabs([...visitedTabs, activeSurfaceTab]);
  }

  return (
    <RoomSidePanelContainer
      ariaLabel={t("room.panels")}
      kind="auxiliary"
      onSidePanelWidthChange={onSidePanelWidthChange}
      onStartSidePanelResize={onStartSidePanelResize}
      sidePanelWidthPercent={sidePanelWidthPercent}
    >
      <RoomAuxiliaryTabContent
        aboutRequest={aboutRequest}
        activeTab={activeSurfaceTab}
        activeWorkspacePath={activeWorkspacePath}
        composerDraftScopeKey={composerDraftScopeKey}
        conversationId={conversationId}
        currentAgent={currentAgent}
        executionResource={executionResource}
        executionTaskRuns={executionTaskRuns}
        isDm={isDm}
        onClose={onClose}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        onSaveAgentOptions={onSaveAgentOptions}
        onValidateAgentName={onValidateAgentName}
        roomId={roomId}
        roomMembers={roomMembers}
        subagentRequest={subagentRequest}
        subagentTaskSource={subagentTaskSource}
        visitedTabs={visitedTabs}
      />
    </RoomSidePanelContainer>
  );
}
