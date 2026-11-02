// INPUT: Room 辅助页栏目键值、当前 Agent/成员、执行资源与文件/配置命令。
// OUTPUT: 统一装配工作图、子智能体、工作区与简介四类 Room 辅助面板。
// POS: 本地与在线 Room（含桌面常驻与窄屏弹层）共用的辅助面板内容层。

import type { ReactNode } from "react";

import { buildExecutionAgentDirectory } from "@/features/conversation/shared/execution/execution-process-model";
import { ExecutionWorkGraphSurface } from "@/features/conversation/shared/execution/execution-workgraph-surface";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { cn } from "@/shared/ui/class-name";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomWorkspaceView } from "../workspace/room-workspace-view";
import type { RoomSurfaceTabKey } from "./header/room-header-tabs";
import type { RoomAgentAboutRequest } from "./layout/room-surface-layout-types";
import { RoomAgentAboutSurface } from "./room-agent-about-surface";
import { RoomSubagentTaskSurface } from "./room-subagent-task-surface";

interface RoomAuxiliaryTabContentProps {
  aboutRequest?: RoomAgentAboutRequest;
  activeTab: RoomSurfaceTabKey;
  activeWorkspacePath: string | null;
  compact?: boolean;
  composerDraftScopeKey: string | null;
  conversationId: string | null;
  currentAgent: Agent;
  executionResource: ExecutionResource;
  executionTaskRuns: ConversationTaskRun[];
  isDm?: boolean;
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
  onValidateAgentName: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
  roomId: string | null;
  roomMembers: Agent[];
  subagentRequest?: {
    hostAgentId: string | null;
    key: number;
    toolUseId: string | null;
  };
  subagentTaskSource?: SubagentTaskSource | null;
  visitedTabs?: RoomSurfaceTabKey[];
}

export function RoomAuxiliaryTabContent({
  aboutRequest,
  activeTab,
  activeWorkspacePath,
  compact = false,
  composerDraftScopeKey,
  conversationId,
  currentAgent,
  executionResource,
  executionTaskRuns,
  isDm = false,
  onClose,
  onOpenWorkspaceFile,
  onSaveAgentOptions,
  onValidateAgentName,
  roomId,
  roomMembers,
  subagentRequest,
  subagentTaskSource = null,
  visitedTabs,
}: RoomAuxiliaryTabContentProps) {
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
          compact={compact}
          composerDraftScopeKey={composerDraftScopeKey}
          isDm={isDm}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          roomMembers={roomMembers}
        />
      ),
    },
    {
      key: "about",
      content: (
        <RoomAgentAboutSurface
          agent={currentAgent}
          conversationId={conversationId}
          isDm={isDm}
          isVisible={activeTab === "about"}
          onSaveAgentOptions={onSaveAgentOptions}
          onValidateAgentName={onValidateAgentName}
          requestKey={aboutRequest?.key}
          requestedAgentId={aboutRequest?.agent_id}
          requestedTab={aboutRequest?.tab}
          roomId={roomId}
          roomMembers={roomMembers}
        />
      ),
    },
  ];

  const subagentContent = activeTab === "subagents" && subagentTaskSource ? (
    <RoomSubagentTaskSurface
      currentAgentId={currentAgent.agent_id}
      layout={compact ? "mobile" : "desktop"}
      onClose={onClose}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      requestKey={subagentRequest?.key}
      requestedHostAgentId={subagentRequest?.hostAgentId}
      requestedTaskToolUseId={subagentRequest?.toolUseId}
      roomMembers={roomMembers}
      source={subagentTaskSource}
    />
  ) : null;

  if (!visitedTabs) {
    if (activeTab === "subagents") {
      return subagentContent;
    }
    return persistentPanels.find((panel) => panel.key === activeTab)?.content ?? null;
  }

  return (
    <>
      {persistentPanels
        .filter((panel) => visitedTabs.includes(panel.key))
        .map((panel) => (
          <div
            key={panel.key}
            className={cn(
              "flex h-full min-h-0 min-w-0 flex-1 flex-col",
              activeTab !== panel.key && "hidden",
            )}
          >
            {panel.content}
          </div>
        ))}

      {subagentContent ? (
        <div className="flex h-full min-h-0 min-w-0 flex-1 flex-col">
          {subagentContent}
        </div>
      ) : null}
    </>
  );
}
