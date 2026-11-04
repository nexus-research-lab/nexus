const ROOM_DIRECTORY_UPDATED_EVENT_NAME = "nexus:room-directory-updated";

interface RoomDirectoryUpdateDetail {
  directoryRevision?: number;
}

/**
 * Room 与 DM 目录有多个缓存消费者，统一事件只表达快照已经失效。
 * directoryRevision 表示失效对应的已提交目录版本；已加载不早于该版本的目录可跳过重读。
 */
export function notifyRoomDirectoryUpdated(directoryRevision?: number): void {
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent<RoomDirectoryUpdateDetail>(
      ROOM_DIRECTORY_UPDATED_EVENT_NAME,
      { detail: { directoryRevision } },
    ));
  }
}

export function subscribeRoomDirectoryUpdates(
  listener: (directoryRevision?: number) => void,
): () => void {
  if (typeof window === "undefined") {
    return () => undefined;
  }
  const handleUpdate = (event: Event) => {
    listener((event as CustomEvent<RoomDirectoryUpdateDetail | null>).detail?.directoryRevision);
  };
  window.addEventListener(ROOM_DIRECTORY_UPDATED_EVENT_NAME, handleUpdate);
  return () => window.removeEventListener(
    ROOM_DIRECTORY_UPDATED_EVENT_NAME,
    handleUpdate,
  );
}
