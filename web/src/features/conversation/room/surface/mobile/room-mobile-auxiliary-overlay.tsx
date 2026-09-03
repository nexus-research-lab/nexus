import { ArrowLeft } from "lucide-react";

import { buildExecutionAgentDirectory } from "@/features/conversation/shared/execution/execution-process-model";
import { ExecutionWorkGraphSurface } from "@/features/conversation/shared/execution/execution-workgraph-surface";
import type { ExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import type { ConversationTaskRun } from "@/features/conversation/shared/todos/todo-projection-model";
import { RoomWorkspaceView } from "@/features/conversation/room/workspace/room-workspace-view";
import { useI18n } from "@/shared/i18n/i18n-context";
import type {
  Agent,
  AgentIdentityDraft,
  AgentNameValidationResult,
  AgentOptions,
} from "@/types/agent/agent";

import { RoomAgentAboutSurface } from "../room-agent-about-surface";

export type RoomMobileAuxiliaryTab = "about" | "workgraph" | "workspace";

interface RoomMobileAuxiliaryOverlayProps {
  activeTab: RoomMobileAuxiliaryTab | null;
  activeWorkspacePath: string | null;
  composerDraftScopeKey: string | null;
  conversationId: string | null;
  currentAgent: Agent;
  executionResource: ExecutionResource;
  executionTaskRuns: ConversationTaskRun[];
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
  onValidateAgentName: (
    name: string,
    agentId?: string,
  ) => Promise<AgentNameValidationResult>;
  roomId: string | null;
  roomMembers: Agent[];
}

export function RoomMobileAuxiliaryOverlay({
  activeTab,
  activeWorkspacePath,
  composerDraftScopeKey,
  conversationId,
  currentAgent,
  executionResource,
  executionTaskRuns,
  isDm,
  onClose,
  onOpenWorkspaceFile,
  onSaveAgentOptions,
  onValidateAgentName,
  roomId,
  roomMembers,
}: RoomMobileAuxiliaryOverlayProps) {
  const { t } = useI18n();
  if (!activeTab) {
    return null;
  }

  const title = activeTab === "workspace"
    ? t("room.workspace")
    : activeTab === "workgraph"
    ? t("room.workgraph")
    : t("room.about");

  return (
    <div className="fixed inset-0 z-50 flex min-h-0 flex-col [background:var(--surface-popover-background)] backdrop-blur-2xl">
      <header className="flex h-[52px] shrink-0 items-center gap-2 border-b divider-subtle px-2 sm:px-3">
        <button
          aria-label={t("common.back")}
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-(--text-strong) transition hover:bg-(--interaction-hover-background)"
          onClick={onClose}
          type="button"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h2 className="truncate text-base font-semibold text-(--text-strong)">
          {title}
        </h2>
      </header>

      <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
        {activeTab === "workgraph" ? (
          <ExecutionWorkGraphSurface
            agents={[
              ...roomMembers.filter((agent) => (
                agent.agent_id !== currentAgent.agent_id
              )),
              currentAgent,
            ]}
            directory={buildExecutionAgentDirectory([
              ...roomMembers.filter((agent) => (
                agent.agent_id !== currentAgent.agent_id
              )),
              currentAgent,
            ])}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            resource={executionResource}
            taskRuns={executionTaskRuns}
          />
        ) : activeTab === "workspace" ? (
          <RoomWorkspaceView
            activeWorkspacePath={activeWorkspacePath}
            agentId={currentAgent.agent_id}
            composerDraftScopeKey={composerDraftScopeKey}
            compact
            isDm={isDm}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            roomMembers={roomMembers}
          />
        ) : (
          <RoomAgentAboutSurface
            agent={currentAgent}
            conversationId={conversationId}
            isVisible
            onSaveAgentOptions={onSaveAgentOptions}
            onValidateAgentName={onValidateAgentName}
            requestedAgentId={currentAgent.agent_id}
            requestedTab="identity"
            roomId={roomId}
            roomMembers={roomMembers}
          />
        )}
      </div>
    </div>
  );
}
