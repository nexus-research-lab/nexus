import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/core/http";
import { notifyRoomDirectoryUpdated } from "@/lib/conversation/room-directory-events";
import type {
  ApiRoomContextAggregate,
  CreateRoomConversationParams,
  CreateRoomParams,
  RoomContextAggregate,
  UpdateRoomConversationParams,
  UpdateRoomParams,
} from "@/types/conversation/room";

import {
  buildCreateRoomBody,
  buildUpdateRoomBody,
  normalizeConversationTitle,
  transformRoomContext,
} from "./room-api-model";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

async function mutateRoomContext(
  path: string,
  method: "DELETE" | "PATCH" | "POST",
  body?: Record<string, unknown>,
): Promise<RoomContextAggregate> {
  const context = await requestApi<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}${path}`,
    {
      method,
      ...(body ? { body: JSON.stringify(body) } : {}),
    },
  );
  notifyRoomDirectoryUpdated();
  return transformRoomContext(context);
}

export async function uploadRoomConversationAttachmentApi(
  roomId: string,
  conversationId: string,
  file: File,
  path?: string,
): Promise<{ path: string; name: string; size: number }> {
  const formData = new FormData();
  formData.append("file", file);
  if (path) {
    formData.append("path", path);
  }
  return requestApi<{ path: string; name: string; size: number }>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/attachments/upload`,
    { body: formData, method: "POST" },
  );
}

export function createRoom(params: CreateRoomParams): Promise<RoomContextAggregate> {
  return mutateRoomContext("/rooms", "POST", buildCreateRoomBody(params));
}

export function updateRoom(
  roomId: string,
  params: UpdateRoomParams,
): Promise<RoomContextAggregate> {
  return mutateRoomContext(
    `/rooms/${encodeURIComponent(roomId)}`,
    "PATCH",
    buildUpdateRoomBody(params),
  );
}

export function createRoomConversation(
  roomId: string,
  params: CreateRoomConversationParams = {},
): Promise<RoomContextAggregate> {
  return mutateRoomContext(
    `/rooms/${encodeURIComponent(roomId)}/conversations`,
    "POST",
    { title: normalizeConversationTitle(params.title) },
  );
}

export function updateRoomConversation(
  roomId: string,
  conversationId: string,
  params: UpdateRoomConversationParams,
): Promise<RoomContextAggregate> {
  return mutateRoomContext(
    `/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`,
    "PATCH",
    { title: params.title },
  );
}

export function deleteRoomConversation(
  roomId: string,
  conversationId: string,
): Promise<RoomContextAggregate> {
  return mutateRoomContext(
    `/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}`,
    "DELETE",
  );
}

export async function closeRoomConversationRuntime(
  roomId: string,
  conversationId: string,
): Promise<void> {
  await requestApi<{ closed: boolean }>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}/conversations/${encodeURIComponent(conversationId)}/close`,
    { method: "POST" },
  );
}

export function addRoomMember(
  roomId: string,
  agentId: string,
): Promise<RoomContextAggregate> {
  return mutateRoomContext(
    `/rooms/${encodeURIComponent(roomId)}/members`,
    "POST",
    { agent_id: agentId },
  );
}

export function removeRoomMember(
  roomId: string,
  agentId: string,
): Promise<RoomContextAggregate> {
  return mutateRoomContext(
    `/rooms/${encodeURIComponent(roomId)}/members/${encodeURIComponent(agentId)}`,
    "DELETE",
  );
}

export async function deleteRoom(roomId: string): Promise<{ success: boolean }> {
  const result = await requestApi<{ success: boolean }>(
    `${AGENT_API_BASE_URL}/rooms/${encodeURIComponent(roomId)}`,
    { method: "DELETE" },
  );
  notifyRoomDirectoryUpdated();
  return result;
}

export async function ensureDirectRoom(
  agentId: string,
): Promise<RoomContextAggregate> {
  const context = await requestApi<ApiRoomContextAggregate>(
    `${AGENT_API_BASE_URL}/rooms/dm/${encodeURIComponent(agentId)}`,
    { method: "GET" },
  );
  notifyRoomDirectoryUpdated();
  return transformRoomContext(context);
}
