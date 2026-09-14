// INPUT: Exact Room conversation data and source-aware file/permission callbacks.
// OUTPUT: Stable live Thread source preserving file path and workspace identity.
// POS: Room Thread publication boundary; callback updates do not replace the source.

import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { useCallback, useEffect, useMemo, useRef } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
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
  messageGroups: Map<string, Message[]>;
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  roomAgentExecutionStateGroups: Map<string, RoomAgentExecutionState[]>;
  sendPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
}

export function useRoomThreadSource({
  agentAvatarMap,
  agentNameMap,
  conversationId,
  messageGroups,
  onOpenWorkspaceFile,
  pendingPermissionGroups,
  pendingSlotGroups,
  roomAgentExecutionStateGroups,
  sendPermissionResponse,
}: UseRoomThreadSourceOptions): void {
  const { closeThread } = useGroupThread();
  const setSource = useRoomThreadLiveStore((state) => state.setSource);
  const clearSource = useRoomThreadLiveStore((state) => state.clearSource);
  const actions = useStableRoomThreadActions({
    onOpenWorkspaceFile,
    sendPermissionResponse,
  });
  const canOpenWorkspaceFile = Boolean(onOpenWorkspaceFile);
  const source = useMemo<RoomThreadLiveSource>(() => ({
    agentAvatarMap,
    agentNameMap,
    messageGroups,
    onOpenWorkspaceFile: canOpenWorkspaceFile
      ? actions.openWorkspaceFile
      : undefined,
    onPermissionResponse: actions.respondPermission,
    pendingPermissionGroups,
    pendingSlotGroups,
    roomAgentExecutionStateGroups,
  }), [
    actions,
    agentAvatarMap,
    agentNameMap,
    canOpenWorkspaceFile,
    messageGroups,
    pendingPermissionGroups,
    pendingSlotGroups,
    roomAgentExecutionStateGroups,
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
  sendPermissionResponse,
}: Pick<
  UseRoomThreadSourceOptions,
  "onOpenWorkspaceFile" | "sendPermissionResponse"
>) {
  const callbacksRef = useRef({
    onOpenWorkspaceFile,
    sendPermissionResponse,
  });
  useEffect(() => {
    callbacksRef.current = {
      onOpenWorkspaceFile,
      sendPermissionResponse,
    };
  }, [onOpenWorkspaceFile, sendPermissionResponse]);

  const openWorkspaceFile = useCallback<WorkspaceFileOpenHandler>((path, workspaceAgentId) => {
    callbacksRef.current.onOpenWorkspaceFile?.(path, workspaceAgentId);
  }, []);
  const respondPermission = useCallback(
    (payload: PermissionDecisionPayload) =>
      callbacksRef.current.sendPermissionResponse(payload),
    [],
  );
  return useMemo(() => ({
    openWorkspaceFile,
    respondPermission,
  }), [openWorkspaceFile, respondPermission]);
}
