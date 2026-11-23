"use client";

/**
 * INPUT: 轮次开始时间与轮次身份。
 * OUTPUT: 活动行的已用秒数与当前趣味动词序号。
 * POS: 运行中活动行的唯一计时源；位置迁移（独立行 → 工具段 → 尾随）不重置计时，也不换词。
 */
import {
  createContext,
  useContext,
  useMemo,
  useSyncExternalStore,
  type ReactNode,
} from "react";

import type { TranslationKey } from "@/shared/i18n/messages";

const LIVE_VERB_KEYS = [
  "message.live_verb_01",
  "message.live_verb_02",
  "message.live_verb_03",
  "message.live_verb_04",
  "message.live_verb_05",
  "message.live_verb_06",
  "message.live_verb_07",
  "message.live_verb_08",
  "message.live_verb_09",
  "message.live_verb_10",
  "message.live_verb_11",
  "message.live_verb_12",
  "message.live_verb_13",
  "message.live_verb_14",
  "message.live_verb_15",
  "message.live_verb_16",
] as const satisfies readonly TranslationKey[];

// 换词节奏：足够慢以免像跑马灯，足够快让长等待有"还在动"的感知。
const VERB_ROTATION_SECONDS = 8;

interface ActivityClockSource {
  seed: number;
  startedAt: number;
}

const ActivityClockContext = createContext<ActivityClockSource | null>(null);

export function MessageActivityClockProvider({
  children,
  roundId,
  startedAt,
}: {
  children: ReactNode;
  roundId: string;
  startedAt: number | null;
}) {
  const source = useMemo(
    () => (startedAt ? { seed: hashSeed(roundId), startedAt } : null),
    [roundId, startedAt],
  );
  return (
    <ActivityClockContext.Provider value={source}>
      {children}
    </ActivityClockContext.Provider>
  );
}

export interface ActivityClockReading {
  elapsedSeconds: number;
  verbKey: TranslationKey;
}

/** 无轮次来源（如 Team 远程等待行）时返回 null，调用方退回静态文案。 */
// eslint-disable-next-line react-refresh/only-export-components
export function useActivityClock(enabled: boolean): ActivityClockReading | null {
  const source = useContext(ActivityClockContext);
  const active = enabled && source !== null;
  const now = useSyncExternalStore(
    active ? subscribeSecondTicker : subscribeNothing,
    active ? readTickerSecond : readIdle,
    active ? readTickerSecond : readIdle,
  );
  if (!active) {
    return null;
  }
  // 时钟漂移或历史时间戳晚于本机时间时从 0 起算，不显示负数。
  const elapsedSeconds = Math.max(0, Math.floor((now - source.startedAt) / 1000));
  const step = Math.floor(elapsedSeconds / VERB_ROTATION_SECONDS);
  return {
    elapsedSeconds,
    verbKey: LIVE_VERB_KEYS[(source.seed + step) % LIVE_VERB_KEYS.length],
  };
}

/* 全部活动行共用一个秒级 ticker：首个订阅者启动，最后一个退订即停，空闲会话不保留计时器。 */
const listeners = new Set<() => void>();
let ticker: ReturnType<typeof setInterval> | null = null;
let currentSecond = 0;

function floorToSecond(time: number): number {
  return Math.floor(time / 1000) * 1000;
}

// ticker 未运行（首帧订阅前）时就地对齐当前秒，避免首帧读到模块加载时的旧时间。
function readTickerSecond(): number {
  if (!ticker) {
    currentSecond = floorToSecond(Date.now());
  }
  return currentSecond;
}

function readIdle(): number {
  return 0;
}

function subscribeSecondTicker(listener: () => void): () => void {
  listeners.add(listener);
  if (!ticker) {
    currentSecond = floorToSecond(Date.now());
    ticker = setInterval(() => {
      currentSecond = floorToSecond(Date.now());
      listeners.forEach((notify) => notify());
    }, 1000);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && ticker) {
      clearInterval(ticker);
      ticker = null;
    }
  };
}

function subscribeNothing(): () => void {
  return () => {};
}

function hashSeed(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
