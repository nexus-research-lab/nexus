import { useSyncExternalStore } from "react";

const ROOM_SNAPSHOT_TOKEN = "__room_snapshot__";
const EMPTY_ROOM_ACTIVITY: ReadonlyMap<string, RoomActivityStatus> = new Map();

type RoomActivityScope = "round" | "agent_round";
export type RoomActivityStatus = "waiting" | "working";

const activeRoundKeysByRoom = new Map<string, Set<string>>();
const pendingInteractionIdsByRoom = new Map<string, Set<string>>();
const listeners = new Set<() => void>();
let roomActivitySnapshot: ReadonlyMap<string, RoomActivityStatus> = EMPTY_ROOM_ACTIVITY;

/**
 * Room WebSocket 生命周期的短期投影。
 *
 * 侧栏目录是持久化数据，不能承担正在执行中的瞬时状态；这里单独保存
 * Room 级活动集合，避免把同一个执行态错误投影到某个 Agent 私聊行。
 */
export function useRoomActivity(): ReadonlyMap<string, RoomActivityStatus> {
  return useSyncExternalStore(
    subscribe,
    getRoomActivity,
    getRoomActivity,
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
    // 自动续跑的 runtime round 与 logical root ID 不同；最后一个 runtime
    // round 收口后，剩余 Agent key 都已失去活跃 root，必须一起清掉。
    if (![...activeKeys].some((key) => key.startsWith("round:"))) {
      for (const key of activeKeys) {
        if (key.startsWith("agent:")) {
          activeKeys.delete(key);
        }
      }
    }
  } else {
    activeKeys.delete(activityKey);
  }

  writeRoundActivity(normalizedRoomId, activeKeys);
}

/** 更新 Room 中单个人工交互请求的等待状态。 */
export function updateRoomInteraction(
  roomId: string | null | undefined,
  requestId: string | null | undefined,
  pending: boolean,
): void {
  const normalizedRoomId = normalize(roomId);
  const normalizedRequestId = normalize(requestId);
  if (!normalizedRoomId || !normalizedRequestId) {
    return;
  }

  const requestIds = new Set(
    pendingInteractionIdsByRoom.get(normalizedRoomId) ?? [],
  );
  if (pending) {
    requestIds.add(normalizedRequestId);
  } else {
    requestIds.delete(normalizedRequestId);
  }
  writePendingInteractions(normalizedRoomId, requestIds);
}

/** 用订阅恢复时的权威 pending 快照替换 Room 执行态与人工交互态。 */
export function replaceRoomActivitySnapshot(
  roomId: string | null | undefined,
  roundId: string | null | undefined,
  hasPendingSlots: boolean,
  pendingInteractionRequestIds: readonly string[] = [],
): void {
  const normalizedRoomId = normalize(roomId);
  if (!normalizedRoomId) {
    return;
  }

  if (hasPendingSlots) {
    const normalizedRoundId = normalize(roundId) || ROOM_SNAPSHOT_TOKEN;
    activeRoundKeysByRoom.set(
      normalizedRoomId,
      new Set([`round:${normalizedRoundId}`]),
    );
  } else {
    activeRoundKeysByRoom.delete(normalizedRoomId);
  }

  replacePendingInteractions(normalizedRoomId, pendingInteractionRequestIds);
  publishRoomActivity();
}

/** 用 Room 全局订阅恢复值替换人工交互态，不触碰各 conversation 的执行槽。 */
export function replaceRoomInteractionSnapshot(
  roomId: string | null | undefined,
  pendingInteractionRequestIds: readonly string[],
): void {
  const normalizedRoomId = normalize(roomId);
  if (!normalizedRoomId) {
    return;
  }
  replacePendingInteractions(normalizedRoomId, pendingInteractionRequestIds);
  publishRoomActivity();
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
  for (const roomId of pendingInteractionIdsByRoom.keys()) {
    if (!roomIds.has(roomId)) {
      pendingInteractionIdsByRoom.delete(roomId);
      changed = true;
    }
  }
  if (changed) {
    publishRoomActivity();
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** 非 React 消费者读取当前 Room 活动态，测试和侧栏投影共用同一快照。 */
export function getRoomActivity(): ReadonlyMap<string, RoomActivityStatus> {
  return roomActivitySnapshot;
}

function writeRoundActivity(roomId: string, nextKeys: Set<string>): void {
  if (nextKeys.size === 0) {
    activeRoundKeysByRoom.delete(roomId);
  } else {
    activeRoundKeysByRoom.set(roomId, nextKeys);
  }
  publishRoomActivity();
}

function writePendingInteractions(roomId: string, requestIds: Set<string>): void {
  if (requestIds.size === 0) {
    pendingInteractionIdsByRoom.delete(roomId);
  } else {
    pendingInteractionIdsByRoom.set(roomId, requestIds);
  }
  publishRoomActivity();
}

function replacePendingInteractions(
  roomId: string,
  pendingInteractionRequestIds: readonly string[],
): void {
  const requestIds = new Set(
    pendingInteractionRequestIds.map(normalize).filter(Boolean),
  );
  if (requestIds.size > 0) {
    pendingInteractionIdsByRoom.set(roomId, requestIds);
  } else {
    pendingInteractionIdsByRoom.delete(roomId);
  }
}

function publishRoomActivity(): void {
  const nextSnapshot = new Map<string, RoomActivityStatus>();
  for (const roomId of activeRoundKeysByRoom.keys()) {
    nextSnapshot.set(roomId, "working");
  }
  for (const roomId of pendingInteractionIdsByRoom.keys()) {
    nextSnapshot.set(roomId, "waiting");
  }
  if (mapsEqual(roomActivitySnapshot, nextSnapshot)) {
    return;
  }
  roomActivitySnapshot = nextSnapshot;
  for (const listener of listeners) {
    listener();
  }
}

function mapsEqual(
  left: ReadonlyMap<string, RoomActivityStatus>,
  right: ReadonlyMap<string, RoomActivityStatus>,
): boolean {
  if (left.size !== right.size) {
    return false;
  }
  for (const [roomId, status] of left) {
    if (right.get(roomId) !== status) {
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
