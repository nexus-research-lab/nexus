import { useCallback, useRef } from "react";

import {
  closeRoomConversationRuntime,
  createRoomConversation,
  deleteRoomConversation,
  notifyRoomDirectoryUpdated,
  updateRoomConversation,
} from "@/lib/api/room-api";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import type { AgentIdentityDraft, AgentOptions } from "@/types/agent/agent";
import type { RoomContextAggregate } from "@/types/conversation/room";

import { saveRoomManagement } from "./room-management-command";

interface UseRoomPageCommandsOptions {
  roomId?: string | null;
  roomMembers: readonly { agent_id: string }[];
  refreshRoomContexts: () => Promise<RoomContextAggregate[]>;
  saveExistingAgentOptions: (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => Promise<void>;
}

interface RoomCommandPolicy {
  refresh: boolean;
}

type CommandOutcome<Result> =
  | { ok: true; value: Result }
  | { ok: false; error: unknown };

const ROOM_COMMAND_POLICIES = {
  mutate: {refresh: true},
  runtime: {refresh: false},
} satisfies Record<string, RoomCommandPolicy>;

export function useRoomPageCommands({
  roomId,
  roomMembers,
  refreshRoomContexts,
  saveExistingAgentOptions,
}: UseRoomPageCommandsOptions) {
  const scopeRef = useRef(roomId ?? null);
  scopeRef.current = roomId ?? null;

  const runRoomCommand = useCallback(async <Result,>(
    policy: RoomCommandPolicy,
    command: (scopeRoomId: string) => Promise<Result>,
  ): Promise<Result | null> => {
    if (!roomId) {
      return null;
    }
    const scopeRoomId = roomId;
    const commandOutcome = await settleCommand(command(scopeRoomId));
    const refreshOutcome = policy.refresh
      ? await settleCommand(refreshRoomContexts())
      : null;

    // 复合写入可能只完成前半段，失败后仍以服务端快照校正页面。
    if (!commandOutcome.ok) {
      throw commandOutcome.error;
    }
    if (refreshOutcome && !refreshOutcome.ok) {
      throw refreshOutcome.error;
    }
    return scopeRef.current === scopeRoomId ? commandOutcome.value : null;
  }, [refreshRoomContexts, roomId]);

  const handleCreateConversation = useCallback(async (title?: string): Promise<string | null> => {
    const context = await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => createRoomConversation(scopeRoomId, {title}),
    );
    return context?.conversation.id ?? null;
  }, [runRoomCommand]);

  const handleDeleteConversation = useCallback(async (
    conversationId: string,
  ): Promise<string | null> => {
    const fallbackContext = await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => deleteRoomConversation(scopeRoomId, conversationId),
    );
    return fallbackContext?.conversation.id ?? null;
  }, [runRoomCommand]);

  const handleCloseConversation = useCallback(async (conversationId: string) => {
    await runRoomCommand(
      ROOM_COMMAND_POLICIES.runtime,
      (scopeRoomId) => closeRoomConversationRuntime(scopeRoomId, conversationId),
    );
  }, [runRoomCommand]);

  const handleUpdateConversationTitle = useCallback(async (
    conversationId: string,
    title: string,
  ) => {
    await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => updateRoomConversation(scopeRoomId, conversationId, {title}),
    );
  }, [runRoomCommand]);

  const handleManageRoom = useCallback(async (submission: RoomDialogSubmission) => {
    await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => saveRoomManagement(
        scopeRoomId,
        roomMembers.map((member) => member.agent_id),
        submission,
      ),
    );
  }, [roomMembers, runRoomCommand]);

  const handleSaveExistingRoomMemberOptions = useCallback(async (
    agentId: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await saveExistingAgentOptions(agentId, title, options, identity);
    if (roomId) {
      await refreshRoomContexts();
    }
  }, [refreshRoomContexts, roomId, saveExistingAgentOptions]);

  const handleRefreshRoomState = useCallback(async () => {
    if (!roomId) {
      return;
    }
    const scopeRoomId = roomId;
    await refreshRoomContexts();
    if (scopeRef.current === scopeRoomId) {
      notifyRoomDirectoryUpdated();
    }
  }, [refreshRoomContexts, roomId]);

  return {
    handleCreateConversation,
    handleDeleteConversation,
    handleCloseConversation,
    handleUpdateConversationTitle,
    handleManageRoom,
    handleSaveExistingRoomMemberOptions,
    handleRefreshRoomState,
  };
}

async function settleCommand<Result>(
  command: Promise<Result>,
): Promise<CommandOutcome<Result>> {
  try {
    return { ok: true, value: await command };
  } catch (error) {
    return { ok: false, error };
  }
}
