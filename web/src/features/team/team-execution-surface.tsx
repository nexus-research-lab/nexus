// INPUT: 当前群内属于本机用户的真实执行绑定、Agent 和选中的 Room 栏目。
// OUTPUT: 复用 Room 工作图、子智能体、工作区与简介的本机执行面板。
// POS: 仅适配身份，不把远程成员 ID 当作本机资源，也不上传本机私有文件。
import { useCallback, useEffect, useMemo, useState } from "react";
import { X } from "lucide-react";
import { useAgentConversation } from "@/hooks/agent/use-agent-conversation";
import { buildRoomSharedSessionKey } from "@/lib/conversation/session-key";
import type { TeamRoomBinding } from "@/lib/api/conversation/team-node-api";
import type { Agent } from "@/types/agent/agent";
import { useAgentStore } from "@/store/agent";
import { useExistingAgentOptionsCommands } from "@/features/agents/options/use-existing-agent-options-commands";
import { refreshHomeDirectory } from "@/features/home/home-directory-resource";
import { ExecutionWorkGraphSurface } from "@/features/conversation/shared/execution/execution-workgraph-surface";
import { useExecutionResource } from "@/features/conversation/shared/execution/use-execution-resource";
import { buildExecutionAgentDirectory } from "@/features/conversation/shared/execution/execution-process-model";
import { projectConversationTaskRuns } from "@/features/conversation/shared/todos/todo-projection-model";
import { RoomSubagentTaskSurface } from "@/features/conversation/room/surface/room-subagent-task-surface";
import { RoomAgentAboutSurface } from "@/features/conversation/room/surface/room-agent-about-surface";
import { RoomAgentSwitcher } from "@/features/conversation/room/surface/room-agent-switcher";
import { RoomWorkspaceView } from "@/features/conversation/room/workspace/room-workspace-view";
import { RoomMobileOverlayFrame } from "@/features/conversation/room/surface/mobile/room-mobile-overlay-frame";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";
import { UiIconButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import { TeamWorkspace } from "./team-workspace";

export function TeamExecutionSurface({ roomId, tab, agents, binding, selectedAgentId, compact, activeWorkspacePath, onOpenWorkspaceFile, onSelectAgent, onClose }: {
  roomId: string;
  tab: RoomSurfaceTabKey; agents: Agent[]; binding?: TeamRoomBinding; selectedAgentId: string;
  compact: boolean; activeWorkspacePath: string | null; onOpenWorkspaceFile: (path: string | null) => void;
  onSelectAgent: (id: string) => void; onClose: () => void;
}) {
  const { t } = useI18n();
  const [localWorkspace, setLocalWorkspace] = useState(false);
  useEffect(() => { if (activeWorkspacePath) setLocalWorkspace(true); }, [activeWorkspacePath]);
  const sharedWorkspace = tab === "workspace" && (!localWorkspace || !agents.length);
  const agent = agents.find((item) => item.agent_id === selectedAgentId);
  const content = <section className="flex h-full min-h-0 flex-col" aria-label={t("room.panels")}>
    <header className="flex h-11 shrink-0 items-center gap-2 border-b divider-subtle px-3">
      {agents.length && !sharedWorkspace ? <RoomAgentSwitcher members={agents} selectedId={selectedAgentId} onSelect={onSelectAgent} variant="panel" /> : null}
      <span className="flex-1 text-xs text-muted">{t(sharedWorkspace ? "team.files_shared" : "team.local_execution")}</span>
      <UiIconButton aria-label={t("common.close")} onClick={onClose} variant="ghost" size="sm"><X className="h-4 w-4" /></UiIconButton>
    </header>
    {tab === "workspace" && agents.length ? <UiTabs activeValue={localWorkspace ? "local" : "shared"} ariaLabel={t("room.workspace")} options={[{value:"shared",label:t("team.files_shared")},{value:"local",label:t("team.files_local")}]} onChange={(value) => {setLocalWorkspace(value === "local"); if(value === "shared") onOpenWorkspaceFile(null);}} /> : null}
    {sharedWorkspace ? <TeamWorkspace key={roomId} roomId={roomId} /> : agent && binding?.local_agent_id === agent.agent_id ? <BoundExecutionSurface key={binding.conversation_id} agent={agent} binding={binding} tab={tab} compact={compact} activeWorkspacePath={activeWorkspacePath} onOpenWorkspaceFile={onOpenWorkspaceFile} onClose={onClose} />
      : <p role="status" className="p-4 text-sm text-muted">{t(tab === "workgraph" || tab === "subagents" ? "team.execution_empty" : "team.local_execution_empty")}</p>}
  </section>;
  return compact ? <RoomMobileOverlayFrame label={t("room.panels")} onClose={onClose}>{content}</RoomMobileOverlayFrame> : content;
}

function BoundExecutionSurface({ agent, binding, tab, compact, activeWorkspacePath, onOpenWorkspaceFile, onClose }: {
  agent: Agent; binding: TeamRoomBinding; tab: RoomSurfaceTabKey; compact: boolean;
  activeWorkspacePath: string | null; onOpenWorkspaceFile: (path: string | null) => void; onClose: () => void;
}) {
  const [revision, setRevision] = useState(0);
  const sessionKey = buildRoomSharedSessionKey(binding.conversation_id);
  const identity = useMemo(() => ({session_key: sessionKey, room_id: binding.room_id, conversation_id: binding.conversation_id, chat_type: "group" as const}), [sessionKey, binding.room_id, binding.conversation_id]);
  const onRoomEvent = useCallback((type: string) => { if (type === "execution_invalidated") setRevision((value) => value + 1); }, []);
  const conversation = useAgentConversation({identity, on_room_event: onRoomEvent});
  const resource = useExecutionResource({sessionKey, invalidationKey: revision});
  const updateAgent = useAgentStore((state) => state.update_agent);
  const options = useExistingAgentOptionsCommands({updateAgent});
  const members = [agent];
  const openFile = (path: string | null, workspaceAgentId?: string | null) => {
    // 不把其他工作区的同名文件误打开成当前执行 Agent 的文件。
    if (workspaceAgentId !== undefined && workspaceAgentId !== agent.agent_id) return;
    onOpenWorkspaceFile(path);
  };
  const panels = {
    workgraph: <ExecutionWorkGraphSurface agents={members} directory={buildExecutionAgentDirectory(members)} resource={resource} taskRuns={projectConversationTaskRuns(conversation.messages, sessionKey)} onOpenWorkspaceFile={openFile} />,
    subagents: <RoomSubagentTaskSurface currentAgentId={agent.agent_id} roomMembers={members} source={{kind: "room", room_id: binding.room_id, conversation_id: binding.conversation_id}} layout={compact ? "mobile" : "desktop"} onOpenWorkspaceFile={openFile} onClose={onClose} />,
    workspace: <RoomWorkspaceView agentId={agent.agent_id} roomMembers={members} isDm={false} compact={compact} activeWorkspacePath={activeWorkspacePath} composerDraftScopeKey={null} onOpenWorkspaceFile={openFile} />,
    about: <RoomAgentAboutSurface agent={agent} roomMembers={members} roomId={binding.room_id} conversationId={binding.conversation_id} isVisible={tab === "about"} onValidateAgentName={options.validateAgentName} onSaveAgentOptions={async (...args) => { await options.saveAgentOptions(...args); refreshHomeDirectory(); }} />,
    chat: null,
  };
  return <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{panels[tab]}</div>;
}
