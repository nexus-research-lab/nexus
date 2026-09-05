"use client";

/**
 * INPUT: Room conversation source, current Room member, and the member catalog.
 * OUTPUT: A read-only subagent surface scoped to the selected calling Agent.
 * POS: Room-owned adapter between the shared subagent resource and member switcher.
 */

import { SubagentTaskSurface } from "@/features/conversation/shared/subagent/subagent-task-surface";
import { subagentTaskSourceKey } from "@/features/conversation/shared/subagent/subagent-task-model";
import { useEffect } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import type { SubagentTaskSource } from "@/types/conversation/subagent-task";

import { RoomAgentSwitcher } from "./room-agent-switcher";

interface RoomSubagentTaskSurfaceProps {
  currentAgentId: string;
  layout?: "desktop" | "mobile";
  onClose: () => void;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
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
  const resetKey = `${subagentTaskSourceKey(source)}:${memberIds.join(",")}`;
  const [selectedAgentId, setSelectedAgentId] = useResettableState(
    initialAgentId,
    resetKey,
  );
  useEffect(() => {
    const requestedAgentId = requestedHostAgentId?.trim() ?? "";
    if (
      requestKey > 0
      && requestedTaskToolUseId
      && memberIds.includes(requestedAgentId)
    ) {
      setSelectedAgentId(requestedAgentId);
    }
  }, [
    memberIds,
    requestKey,
    requestedHostAgentId,
    requestedTaskToolUseId,
    setSelectedAgentId,
  ]);
  const isRoomSource = source.kind === "room";
  const agentSwitcher = isRoomSource && roomMembers.length > 1 ? (
    <RoomAgentSwitcher
      ariaLabel={t("subagents.switch_caller")}
      members={roomMembers}
      onSelect={setSelectedAgentId}
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
      requestKey={requestKey}
      requestedTaskToolUseId={requestedTaskToolUseId}
      source={source}
    />
  );
}
