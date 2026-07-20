import { useSyncExternalStore } from "react";

const ROOM_SNAPSHOT_TOKEN = "__room_snapshot__";
const EMPTY_ROOM_IDS: ReadonlySet<string> = new Set();

type RoomActivityScope = "round" | "agent_round";

const activeRoundKeysByRoom = new Map<string, Set<string>>();
const listeners = new Set<() => void>();
let activeRoomIdsSnapshot: ReadonlySet<string> = EMPTY_ROOM_IDS;

/**
 * Room WebSocket 生命周期的短期投影。
 *
 * 侧栏目录是持久化数据，不能承担正在执行中的瞬时状态；这里单独保存
 * Room 级活动集合，避免把同一个执行态错误投影到某个 Agent 私聊行。
 */
export function useActiveRoomIds(): ReadonlySet<string> {
  return useSyncExternalStore(
    subscribe,
    getActiveRoomIds,
    getActiveRoomIds,
  );
}

/** 更新 Room root 或 Agent slot 的生命周期。 */
export function updateRoomActivity(
  roomId: string | null | undefined,
  roundId: string | null | undefined,
  status: string | null | undefined,
  scope: RoomActivityScope = "round",
  agentRoundId?: string | null,
): void {
  const normalizedRoomId = normalize(roomId);
  const normalizedRoundId = normalize(roundId) || ROOM_SNAPSHOT_TOKEN;
  const normalizedStatus = normalize(status);
  if (!normalizedRoomId || !isKnownRoundStatus(normalizedStatus)) {
    return;
  }

  const activeKeys = activeRoundKeysByRoom.get(normalizedRoomId) ?? new Set<string>();
  activeKeys.delete(ROOM_SNAPSHOT_TOKEN);
  const activityKey = scope === "round"
    ? `round:${normalizedRoundId}`
    : `agent:${normalizedRoundId}:${normalize(agentRoundId) || ROOM_SNAPSHOT_TOKEN}`;

  if (normalizedStatus === "running") {
    activeKeys.add(activityKey);
  } else if (scope === "round") {
    // root 已结束时，兜底清理同一 root 下遗漏的 slot 终态事件。
    for (const key of activeKeys) {
      if (key === activityKey || key.startsWith(`agent:${normalizedRoundId}:`)) {
        activeKeys.delete(key);
      }
    }
  } else {
    activeKeys.delete(activityKey);
  }

  writeRoomActivity(normalizedRoomId, activeKeys);
}

/** 用订阅恢复时的权威 pending 快照替换 Room 活动态。 */
export function replaceRoomActivitySnapshot(
  roomId: string | null | undefined,
  roundId: string | null | undefined,
  hasPendingSlots: boolean,
): void {
  const normalizedRoomId = normalize(roomId);
  if (!normalizedRoomId) {
    return;
  }
  if (!hasPendingSlots) {
    writeRoomActivity(normalizedRoomId, new Set());
    return;
  }

  const normalizedRoundId = normalize(roundId) || ROOM_SNAPSHOT_TOKEN;
  writeRoomActivity(normalizedRoomId, new Set([`round:${normalizedRoundId}`]));
}

/** 目录变化后清理已不存在的 Room，避免活动态集合无限增长。 */
export function pruneRoomActivity(roomIds: ReadonlySet<string>): void {
  let changed = false;
  for (const roomId of activeRoundKeysByRoom.keys()) {
    if (!roomIds.has(roomId)) {
      activeRoundKeysByRoom.delete(roomId);
      changed = true;
    }
  }
  if (changed) {
    publishActiveRoomIds();
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getActiveRoomIds(): ReadonlySet<string> {
  return activeRoomIdsSnapshot;
}

function writeRoomActivity(roomId: string, nextKeys: Set<string>): void {
  if (nextKeys.size === 0) {
    activeRoundKeysByRoom.delete(roomId);
  } else {
    activeRoundKeysByRoom.set(roomId, nextKeys);
  }
  publishActiveRoomIds();
}

function publishActiveRoomIds(): void {
  const nextSnapshot = new Set(activeRoundKeysByRoom.keys());
  if (setsEqual(activeRoomIdsSnapshot, nextSnapshot)) {
    return;
  }
  activeRoomIdsSnapshot = nextSnapshot;
  for (const listener of listeners) {
    listener();
  }
}

function setsEqual(left: ReadonlySet<string>, right: ReadonlySet<string>): boolean {
  if (left.size !== right.size) {
    return false;
  }
  for (const value of left) {
    if (!right.has(value)) {
      return false;
    }
  }
  return true;
}

function isKnownRoundStatus(value: string): boolean {
  return value === "running"
    || value === "finished"
    || value === "interrupted"
    || value === "error";
}

function normalize(value: string | null | undefined): string {
  return value?.trim() ?? "";
}
