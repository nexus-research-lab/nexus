/**
 * [INPUT]: Room 与 Conversation 标识，以及资源卡片携带的一次性开场动作。
 * [OUTPUT]: 稳定的一次性动作键，并在当前浏览器中记录是否已经成功消费。
 * [POS]: Room 创建卡片与目标会话之间的幂等边界，防止历史卡片重复启动协作。
 */

const ROOM_INITIAL_ACTION_STORAGE_PREFIX = "nexus.room_initial_action_consumed";

export function buildRoomInitialActionKey(
  roomId: string,
  conversationId?: string | null,
): string {
  return [roomId.trim(), conversationId?.trim() || "main"].join(":");
}

export function isRoomInitialActionConsumed(actionKey: string): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    return window.localStorage.getItem(
      `${ROOM_INITIAL_ACTION_STORAGE_PREFIX}:${actionKey}`,
    ) === "true";
  } catch {
    return false;
  }
}

export function markRoomInitialActionConsumed(actionKey: string): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(
      `${ROOM_INITIAL_ACTION_STORAGE_PREFIX}:${actionKey}`,
      "true",
    );
  } catch {
    // 浏览器禁止持久化时仍允许进入 Room；本次页面内的状态会阻止重复发送。
  }
}
