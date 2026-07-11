import { useCallback, useRef } from "react";

import {
  addRoomMember,
  closeRoomConversationRuntime,
  createRoomConversation,
  deleteRoomConversation,
  notifyRoomDirectoryUpdated,
  removeRoomMember,
  updateRoom,
  updateRoomConversation,
} from "@/lib/api/room-api";
import type { AgentIdentityDraft, AgentOptions } from "@/types/agent/agent";
import type { RoomContextAggregate, UpdateRoomParams } from "@/types/conversation/room";

interface UseRoomPageCommandsOptions {
  roomId?: string | null;
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

const ROOM_COMMAND_POLICIES = {
  mutate: {refresh: true},
  runtime: {refresh: false},
} satisfies Record<string, RoomCommandPolicy>;

export function useRoomPageCommands({
  roomId,
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
    const result = await command(scopeRoomId);
    if (policy.refresh) {
      await refreshRoomContexts();
    }
    return scopeRef.current === scopeRoomId ? result : null;
  }, [refreshRoomContexts, roomId]);

  const handleUpdateRoom = useCallback(async (params: UpdateRoomParams) => {
    await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => updateRoom(scopeRoomId, params),
    );
  }, [runRoomCommand]);

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

  const handleAddRoomMember = useCallback(async (agentId: string) => {
    await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => addRoomMember(scopeRoomId, agentId),
    );
  }, [runRoomCommand]);

  const handleRemoveRoomMember = useCallback(async (agentId: string) => {
    await runRoomCommand(
      ROOM_COMMAND_POLICIES.mutate,
      (scopeRoomId) => removeRoomMember(scopeRoomId, agentId),
    );
  }, [runRoomCommand]);

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
    handleUpdateRoom,
    handleCreateConversation,
    handleDeleteConversation,
    handleCloseConversation,
    handleUpdateConversationTitle,
    handleAddRoomMember,
    handleRemoveRoomMember,
    handleSaveExistingRoomMemberOptions,
    handleRefreshRoomState,
  };
}
