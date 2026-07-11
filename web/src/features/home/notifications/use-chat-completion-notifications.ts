import { useCallback, useEffect, useMemo, useRef } from "react";
import { useLocation } from "react-router-dom";

import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { useSidebarStore } from "@/store/sidebar";
import type { AssistantMessage, EventMessage } from "@/types/conversation/message";

import {
  isWindowActive,
  showBrowserNotification,
  subscribeBrowserNotificationPermission,
} from "./browser-notification";
import {
  buildChatNotificationDirectoryIndex,
  buildMessageNotificationTarget,
  buildNotificationContent,
  getNotificationMessageId,
  toChatNotificationTargetState,
  type ChatNotificationDirectoryIndex,
} from "./chat-notification-model";
import {
  getActiveChatTargetFromPath,
  isChatNotificationTargetActive,
  type ActiveChatNotificationTarget,
} from "./chat-notification-target";
import { useChatNotificationSocket } from "./use-chat-notification-socket";

export function useChatCompletionNotifications(): void {
  const location = useLocation();
  const directory = useHomeDirectory();
  const directoryIndex = useMemo(
    () => buildChatNotificationDirectoryIndex(directory),
    [directory],
  );
  const directoryIndexRef = useRef(directoryIndex);
  directoryIndexRef.current = directoryIndex;
  const activeTargetRef = useRef<ActiveChatNotificationTarget | null>(null);
  activeTargetRef.current = getActiveChatTargetFromPath(location.pathname);
  const recordNotification = useSidebarStore((state) => state.record_chat_notification);
  const clearTarget = useSidebarStore(
    (state) => state.clear_chat_notifications_for_target,
  );
  const clearRoom = useSidebarStore(
    (state) => state.clear_chat_notifications_for_room,
  );

  const clearRoomNotifications = useCallback((roomId: string | null | undefined) => {
    if (!roomId) {
      return;
    }
    clearRoom(roomId);
    for (const targetKey of getRoomSessionTargetKeys(directoryIndexRef.current, roomId)) {
      clearTarget(targetKey);
    }
  }, [clearRoom, clearTarget]);

  const clearActiveNotifications = useCallback(() => {
    if (!isWindowActive()) {
      return;
    }
    const activeTarget = activeTargetRef.current;
    if (activeTarget?.room_id) {
      clearRoomNotifications(activeTarget.room_id);
    } else {
      clearTarget(activeTarget?.key);
    }
  }, [clearRoomNotifications, clearTarget]);

  useEffect(clearActiveNotifications, [clearActiveNotifications, directoryIndex, location.pathname]);
  useEffect(() => subscribeBrowserNotificationPermission(), []);
  useEffect(() => {
    window.addEventListener("focus", clearActiveNotifications);
    document.addEventListener("visibilitychange", clearActiveNotifications);
    return () => {
      window.removeEventListener("focus", clearActiveNotifications);
      document.removeEventListener("visibilitychange", clearActiveNotifications);
    };
  }, [clearActiveNotifications]);

  const handleCompletedMessage = useCallback((
    event: EventMessage,
    message: AssistantMessage,
  ) => {
    const index = directoryIndexRef.current;
    const target = buildMessageNotificationTarget(event, message, index);
    if (!target) {
      return;
    }
    if (isChatNotificationTargetActive(activeTargetRef.current, target) && isWindowActive()) {
      if (target.room_id) {
        clearRoomNotifications(target.room_id);
      } else {
        clearTarget(target.key);
      }
      return;
    }
    const messageId = getNotificationMessageId(event, message, target.key);
    if (!recordNotification(toChatNotificationTargetState(target), messageId)) {
      return;
    }
    const { body, title } = buildNotificationContent(target, message, index);
    showBrowserNotification(title, body, messageId);
  }, [clearRoomNotifications, clearTarget, recordNotification]);

  const roomIdsKey = useMemo(
    () => directory.rooms.map((room) => room.id).filter(Boolean).sort().join("\n"),
    [directory.rooms],
  );
  useChatNotificationSocket({ onCompletedMessage: handleCompletedMessage, roomIdsKey });
}

function getRoomSessionTargetKeys(
  index: ChatNotificationDirectoryIndex,
  roomId: string,
): string[] {
  return index.sessionTargetKeysByRoomId.get(roomId) ?? [];
}
