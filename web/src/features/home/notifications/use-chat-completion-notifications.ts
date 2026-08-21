/**
 * INPUT: 全局聊天完成事件、共享聊天目录、当前路由与窗口可见性。
 * OUTPUT: 浏览器通知、侧栏未读数字及用于选择目标 Conversation 的消息顺序。
 * POS: 聊天完成通知副作用入口；当前可见目标直接确认，不向 Feed 注入未读定位。
 */
import { useCallback, useEffect, useMemo, useRef } from "react";
import { useLocation } from "react-router-dom";

import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { hasVisibleAssistantOutput } from "@/features/conversation/shared/message/message-content-model";
import { useSidebarStore } from "@/store/sidebar";
import type { AssistantMessage } from "@/types/conversation/message/entity";
import type { EventMessage } from "@/types/generated/protocol";

import {
  isWindowActive,
  showBrowserNotification,
  subscribeBrowserNotificationPermission,
} from "./browser-notification";
import {
  buildChatNotificationDirectoryIndex,
  getRoomSessionTargetKeys,
} from "./chat-notification-directory";
import {
  buildMessageNotificationTarget,
  buildNotificationContent,
  getNotificationMessageId,
  toChatNotificationTargetState,
} from "./chat-notification-model";
import {
  getActiveChatTargetFromPath,
  isChatNotificationTargetActive,
  isGroupRoomNotificationTarget,
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
  const discardRoomChatState = useSidebarStore(
    (state) => state.discard_chat_state_for_room,
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
      const activeRoom = directoryIndexRef.current.roomsById.get(
        activeTarget.room_id,
      );
      if (!activeRoom || activeRoom.room_type === "room") {
        clearTarget(activeTarget.key);
        return;
      }
      clearRoomNotifications(activeTarget.room_id);
    } else {
      clearTarget(activeTarget?.key);
    }
  }, [clearRoomNotifications, clearTarget]);

  useEffect(clearActiveNotifications, [clearActiveNotifications, directoryIndex, location.pathname]);
  useEffect(() => subscribeBrowserNotificationPermission(), []);
  useEffect(() => {
    if (!directory.hasLoaded) {
      return;
    }
    const knownRoomIds = new Set(directory.rooms.map((room) => room.id));
    const state = useSidebarStore.getState();
    const staleRoomIds = new Set(
      [
        ...Object.values(state.chat_unread_targets),
        ...Object.values(state.chat_unread_anchors),
      ].flatMap((target) => (
        target.room_id && !knownRoomIds.has(target.room_id)
          ? [target.room_id]
          : []
      )),
    );
    for (const roomId of staleRoomIds) {
      discardRoomChatState(roomId);
    }
  }, [
    directory.hasLoaded,
    directory.rooms,
    discardRoomChatState,
  ]);
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
    const isActive = isChatNotificationTargetActive(
      activeTargetRef.current,
      target,
    ) && isWindowActive();
    const isGroupRoom = isGroupRoomNotificationTarget(
      target,
      target.room_id
        ? index.roomsById.get(target.room_id)?.room_type
        : null,
    );
    if (isGroupRoom && !hasVisibleAssistantOutput(message)) {
      return;
    }
    if (isActive && !isGroupRoom) {
      if (target.room_id) {
        clearRoomNotifications(target.room_id);
      } else {
        clearTarget(target.key);
      }
      return;
    }
    if (isActive) {
      clearTarget(target.key);
      return;
    }
    const messageId = getNotificationMessageId(event, message, target.key);
    const didRecord = recordNotification(toChatNotificationTargetState(target), {
      agent_id: message.agent_id,
      agent_round_id: message.agent_round_id ?? event.agent_round_id,
      message_id: messageId,
      room_seq: event.room_seq,
      round_id: message.round_id || event.round_id,
      timestamp: message.timestamp || event.timestamp,
    }, {
      preserve_anchor: isGroupRoom,
    });
    if (!didRecord) {
      return;
    }
    if (isActive) {
      return;
    }
    const { body, title } = buildNotificationContent(target, message, index);
    showBrowserNotification(title, body, messageId);
  }, [clearRoomNotifications, clearTarget, recordNotification]);

  const roomIdsKey = useMemo(
    () => directory.rooms.map((room) => room.id).filter(Boolean).sort().join("\n"),
    [directory.rooms],
  );
  useChatNotificationSocket({
    directoryIndex,
    onCompletedMessage: handleCompletedMessage,
    onRoomDeleted: discardRoomChatState,
    roomIdsKey,
  });
}
