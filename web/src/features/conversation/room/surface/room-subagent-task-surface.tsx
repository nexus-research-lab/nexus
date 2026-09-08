"use client";

/**
 * INPUT: Room conversation source, current Room member, and the member catalog.
 * OUTPUT: Caller-scoped tasks with one-shot exact navigation; manual selection survives catalog reorder.
 * POS: Room-owned adapter between the shared subagent resource and member switcher.
 */

import { SubagentTaskSurface } from "@/features/conversation/shared/subagent/subagent-task-surface";
import { subagentTaskSourceKey } from "@/features/conversation/shared/subagent/subagent-task-model";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomAgentSwitcher } from "./room-agent-switcher";

interface RoomSubagentTaskSurfaceProps {
  currentAgentId: string;
  layout?: "desktop" | "mobile";
  onClose: () => void;
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
  requestKey?: number;
  requestedHostAgentId?: string | null;
  requestedTaskToolUseId?: string | null;
  roomMembers: Agent[];
  source: SubagentTaskSource;
}

export function RoomSubagentTaskSurface({
  currentAgentId,
  layout = "desktop",
  onClose,
  onOpenWorkspaceFile,
  requestKey = 0,
  requestedHostAgentId,
  requestedTaskToolUseId,
  roomMembers,
  source,
}: RoomSubagentTaskSurfaceProps) {
  const { t } = useI18n();
  const memberIds = roomMembers
    .map((member) => member.agent_id.trim())
    .filter(Boolean);
  const initialAgentId = memberIds.includes(currentAgentId)
    ? currentAgentId
    : (memberIds[0] ?? "");
  const requestedAgentId = requestedHostAgentId?.trim() ?? "";
  const requestedToolUseId = requestedTaskToolUseId?.trim() ?? "";
  const sourceKey = subagentTaskSourceKey(source);
  const requestIdentity = JSON.stringify([requestKey, requestedAgentId, requestedToolUseId]);
  const [requestSourceKey, setRequestSourceKey] = useResettableState<string | null>(sourceKey, requestIdentity);
  // A source change consumes the old intent permanently, including on return.
  if (requestSourceKey !== null && requestSourceKey !== sourceKey) setRequestSourceKey(null);
  const validRequest = requestSourceKey === sourceKey
    && Number.isSafeInteger(requestKey) && requestKey > 0 && Boolean(requestedToolUseId);
  const resetKey = JSON.stringify([sourceKey, requestIdentity]);
  // Null means the current request/default still owns selection. A manual choice
  // consumes that request, including a target whose task has not arrived yet.
  const [manualAgentId, setManualAgentId] = useResettableState<string | null>(null, resetKey);
  const requestedCallerReady = validRequest && memberIds.includes(requestedAgentId);
  const selectedAgentId = manualAgentId !== null
    ? memberIds.includes(manualAgentId) ? manualAgentId : initialAgentId
    : requestedCallerReady ? requestedAgentId : initialAgentId;
  if (manualAgentId !== null && manualAgentId !== selectedAgentId) {
    setManualAgentId(selectedAgentId);
  }
  const isRoomSource = source.kind === "room";
  const followRequestedTask = manualAgentId === null && validRequest
    && (!isRoomSource || requestedCallerReady);
  const agentSwitcher = isRoomSource && roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      ariaLabel={t("subagents.switch_caller")}
      members={roomMembers}
      onSelect={setManualAgentId}
      selectedId={selectedAgentId}
      variant="panel"
    />
  ) : null;

  return (
    <SubagentTaskSurface
      headerLeading={agentSwitcher}
      hostAgentId={isRoomSource ? selectedAgentId : null}
      layout={layout}
      onClose={onClose}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      requestKey={followRequestedTask ? requestKey : 0}
      requestedTaskToolUseId={followRequestedTask ? requestedToolUseId : null}
      source={source}
    />
  );
}
