import { useMemo } from "react";

import { getExternalSessionKeyFromConversationId } from "@/lib/conversation/external-session";
import type { Agent } from "@/types/agent/agent";
import type { RoomContextAggregate } from "@/types/conversation/room";

import { useRoomExternalSessions } from "../use-room-external-sessions";
import {
  buildRoomPageBaseModel,
  buildRoomPageModel,
  type RoomPageModel,
} from "./room-page-model";

interface UseRoomPageModelOptions {
  agents: Agent[];
  conversationId?: string | null;
  preferredConversationId?: string | null;
  roomContexts: RoomContextAggregate[];
  roomId?: string | null;
  sessionKey?: string | null;
}

function normalizeRouteSessionKey(value: string | null | undefined): string | null {
  return value?.trim() || null;
}

function getExternalAgentId(base: ReturnType<typeof buildRoomPageBaseModel>): string | null {
  return base.currentAgent?.agent_id ?? null;
}

function getExternalRoomId(base: ReturnType<typeof buildRoomPageBaseModel>): string | null {
  return base.currentRoom?.id ?? null;
}

function getExternalRoomType(base: ReturnType<typeof buildRoomPageBaseModel>): string | null {
  return base.currentRoom?.room_type ?? null;
}

export function useRoomPageModel({
  agents,
  conversationId,
  preferredConversationId,
  roomContexts,
  roomId,
  sessionKey,
}: UseRoomPageModelOptions): RoomPageModel {
  const base = useMemo(
    () => buildRoomPageBaseModel({
      agents,
      conversationId,
      preferredConversationId,
      roomContexts,
      roomId,
    }),
    [agents, conversationId, preferredConversationId, roomContexts, roomId],
  );
  const routeSessionKey = normalizeRouteSessionKey(sessionKey);
  const externalSessions = useRoomExternalSessions({
    agentId: getExternalAgentId(base),
    roomId: getExternalRoomId(base),
    roomType: getExternalRoomType(base),
  });
  const isSelectionReady = (
    !preferredConversationId
    || !getExternalSessionKeyFromConversationId(preferredConversationId)
    || externalSessions.isExternalSessionCatalogReady
  );

  return useMemo(
    () => buildRoomPageModel({
      base,
      externalAgentSessions: externalSessions.externalAgentSessions,
      externalRoomConversations: externalSessions.externalRoomConversations,
      isSelectionReady,
      preferredConversationId: preferredConversationId ?? null,
      routeRoomId: roomId ?? null,
      routeSessionKey,
    }),
    [
      base,
      externalSessions.externalAgentSessions,
      externalSessions.externalRoomConversations,
      isSelectionReady,
      preferredConversationId,
      roomId,
      routeSessionKey,
    ],
  );
}
