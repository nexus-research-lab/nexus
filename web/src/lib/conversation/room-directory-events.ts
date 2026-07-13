const ROOM_DIRECTORY_UPDATED_EVENT_NAME = "nexus:room-directory-updated";

/** Room 与 DM 目录有多个缓存消费者，统一事件只表达快照已经失效。 */
export function notifyRoomDirectoryUpdated(): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent(ROOM_DIRECTORY_UPDATED_EVENT_NAME));
  }
}

export function subscribeRoomDirectoryUpdates(
  listener: () => void,
): () => void {
  if (typeof window === "undefined") {
    return () => undefined;
  }
  window.addEventListener(ROOM_DIRECTORY_UPDATED_EVENT_NAME, listener);
  return () => window.removeEventListener(
    ROOM_DIRECTORY_UPDATED_EVENT_NAME,
    listener,
  );
}
