import { useCallback, useEffect, useMemo, useRef } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import { useGroupThread } from "../group-thread-state";
import {
  useRoomThreadLiveStore,
  type RoomThreadLiveSource,
} from "./room-thread-live-store";

interface UseRoomThreadSourceOptions {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  conversationId: string | null;
  currentUserAvatar: string | null;
  messageGroups: Map<string, Message[]>;
  onOpenWorkspaceFile?: (path: string) => void;
  onStopMessage: (msgId: string) => void;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  sendPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
}

export function useRoomThreadSource({
  agentAvatarMap,
  agentNameMap,
  conversationId,
  currentUserAvatar,
  messageGroups,
  onOpenWorkspaceFile,
  onStopMessage,
  pendingPermissionGroups,
  pendingSlotGroups,
  sendPermissionResponse,
}: UseRoomThreadSourceOptions): void {
  const { closeThread } = useGroupThread();
  const setSource = useRoomThreadLiveStore((state) => state.setSource);
  const clearSource = useRoomThreadLiveStore((state) => state.clearSource);
  const actions = useStableRoomThreadActions({
    onOpenWorkspaceFile,
    onStopMessage,
    sendPermissionResponse,
  });
  const canOpenWorkspaceFile = Boolean(onOpenWorkspaceFile);
  const source = useMemo<RoomThreadLiveSource>(() => ({
    agentAvatarMap,
    agentNameMap,
    currentUserAvatar,
    messageGroups,
    onOpenWorkspaceFile: canOpenWorkspaceFile
      ? actions.openWorkspaceFile
      : undefined,
    onPermissionResponse: actions.respondPermission,
    onStopMessage: actions.stopMessage,
    pendingPermissionGroups,
    pendingSlotGroups,
  }), [
    actions,
    agentAvatarMap,
    agentNameMap,
    canOpenWorkspaceFile,
    currentUserAvatar,
    messageGroups,
    pendingPermissionGroups,
    pendingSlotGroups,
  ]);

  useEffect(() => {
    closeThread();
  }, [closeThread, conversationId]);
  useEffect(() => {
    setSource(source);
  }, [setSource, source]);
  useEffect(() => () => clearSource(), [clearSource]);
}

function useStableRoomThreadActions({
  onOpenWorkspaceFile,
  onStopMessage,
  sendPermissionResponse,
}: Pick<
  UseRoomThreadSourceOptions,
  "onOpenWorkspaceFile" | "onStopMessage" | "sendPermissionResponse"
>) {
  const callbacksRef = useRef({
    onOpenWorkspaceFile,
    onStopMessage,
    sendPermissionResponse,
  });
  useEffect(() => {
    callbacksRef.current = {
      onOpenWorkspaceFile,
      onStopMessage,
      sendPermissionResponse,
    };
  }, [onOpenWorkspaceFile, onStopMessage, sendPermissionResponse]);

  const openWorkspaceFile = useCallback((path: string) => {
    callbacksRef.current.onOpenWorkspaceFile?.(path);
  }, []);
  const respondPermission = useCallback(
    (payload: PermissionDecisionPayload) =>
      callbacksRef.current.sendPermissionResponse(payload),
    [],
  );
  const stopMessage = useCallback((msgId: string) => {
    callbacksRef.current.onStopMessage(msgId);
  }, []);

  return useMemo(() => ({
    openWorkspaceFile,
    respondPermission,
    stopMessage,
  }), [openWorkspaceFile, respondPermission, stopMessage]);
}
