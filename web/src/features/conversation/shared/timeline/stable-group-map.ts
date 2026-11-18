/**
 * INPUT: 上一次与本次按轮次/节点分组的数组 Map。
 * OUTPUT: 元素逐个同一引用的分组复用上一次数组，其余保持本次结果。
 * POS: 流式帧只替换变化的消息；未变化的轮次与 Agent 节点保持数组身份，让 memo 视图跳过重渲染。
 */
import { useRef } from "react";

export function stabilizeGroupMap<T>(
  previous: ReadonlyMap<string, readonly T[]> | null,
  next: Map<string, T[]>,
): Map<string, T[]> {
  if (!previous || previous.size === 0) {
    return next;
  }
  for (const [key, items] of next) {
    const prior = previous.get(key);
    if (prior && prior !== items && hasSameItems(prior, items)) {
      next.set(key, prior as T[]);
    }
  }
  return next;
}

function hasSameItems<T>(left: readonly T[], right: readonly T[]): boolean {
  if (left.length !== right.length) {
    return false;
  }
  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) {
      return false;
    }
  }
  return true;
}

/** 渲染期复用只依赖逐元素同一引用；被丢弃的并发渲染留下的上次结果仍满足同一判定，不会复用过期内容。 */
export function useStableGroupMap<T>(next: Map<string, T[]>): Map<string, T[]> {
  const previousRef = useRef<Map<string, T[]> | null>(null);
  const stable = stabilizeGroupMap(previousRef.current, next);
  previousRef.current = stable;
  return stable;
}
