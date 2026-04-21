/**
 * useFollowScroll — 自动跟随底部的滚动管理 hook
 *
 * 封装聊天面板的滚动控制逻辑：
 * - 新消息 / loading 时自动滚到底部
 * - 用户上滚时暂停跟随
 * - 内容 resize 时保持位置
 * - 支持触摸手势取消自动跟随
 */

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

const BOTTOM_THRESHOLD_PX = 80;
const SMOOTH_SCROLL_DURATION_MS = 420;
const EASE_X1 = 0.23;
const EASE_Y1 = 1;
const EASE_X2 = 0.32;
const EASE_Y2 = 1;

function sample_cubic(a: number, b: number, c: number, t: number): number {
  return ((a * t + b) * t + c) * t;
}

function sample_cubic_derivative(
  a: number,
  b: number,
  c: number,
  t: number,
): number {
  return (3 * a * t + 2 * b) * t + c;
}

function solve_bezier_progress(progress: number): number {
  const clamped_progress = Math.min(Math.max(progress, 0), 1);
  const cx = 3 * EASE_X1;
  const bx = 3 * (EASE_X2 - EASE_X1) - cx;
  const ax = 1 - cx - bx;
  const cy = 3 * EASE_Y1;
  const by = 3 * (EASE_Y2 - EASE_Y1) - cy;
  const ay = 1 - cy - by;

  let t = clamped_progress;
  for (let iteration = 0; iteration < 5; iteration += 1) {
    const x = sample_cubic(ax, bx, cx, t) - clamped_progress;
    const derivative = sample_cubic_derivative(ax, bx, cx, t);
    if (Math.abs(derivative) < 1e-6) {
      break;
    }
    t -= x / derivative;
  }

  let lower = 0;
  let upper = 1;
  t = Math.min(Math.max(t, 0), 1);
  for (let iteration = 0; iteration < 8; iteration += 1) {
    const x = sample_cubic(ax, bx, cx, t);
    if (Math.abs(x - clamped_progress) < 1e-5) {
      break;
    }
    if (x > clamped_progress) {
      upper = t;
    } else {
      lower = t;
    }
    t = (lower + upper) / 2;
  }

  return sample_cubic(ay, by, cy, t);
}

interface UseFollowScrollOptions {
  /** 消息数量变化时触发滚动 */
  message_count: number;
  /** 权限/插槽块变化时触发滚动 */
  auxiliary_block_count?: number;
  /** loading 变化时触发滚动 */
  is_loading: boolean;
  /** session 切换时重置跟随状态 */
  session_key: string | null;
  /** 历史消息 prepend 完成后，用于恢复滚动锚点 */
  history_prepend_token?: number;
}

interface UseFollowScrollReturn {
  /** 挂载到滚动容器的 ref */
  scroll_ref: React.RefObject<HTMLDivElement | null>;
  /** 挂载到 feed 内容区的 ref（ResizeObserver 用） */
  feed_ref: React.RefObject<HTMLDivElement | null>;
  /** 底部锚点 ref */
  bottom_anchor_ref: React.RefObject<HTMLDivElement | null>;
  /** 是否显示"回到底部"按钮 */
  show_scroll_to_bottom: boolean;
  /** 滚动到底部 */
  scroll_to_bottom: (behavior?: ScrollBehavior) => void;
  /** 在 prepend 历史消息前记录当前滚动锚点 */
  prepare_history_prepend_restore: () => void;
  /** prepend 被取消或失败时清理锚点 */
  cancel_history_prepend_restore: () => void;
  /** 事件处理器：挂载到滚动容器的 onScroll */
  on_scroll: () => void;
  /** 事件处理器：挂载到滚动容器的 onWheel */
  on_wheel: (event: React.WheelEvent<HTMLDivElement>) => void;
  /** 事件处理器：挂载到滚动容器的 onTouchStart */
  on_touch_start: (event: React.TouchEvent<HTMLDivElement>) => void;
  /** 事件处理器：挂载到滚动容器的 onTouchMove */
  on_touch_move: (event: React.TouchEvent<HTMLDivElement>) => void;
  /** 事件处理器：挂载到滚动容器的 onTouchEnd */
  on_touch_end: () => void;
}

export function useFollowScroll({
  message_count,
  auxiliary_block_count = 0,
  is_loading,
  session_key,
  history_prepend_token = 0,
}: UseFollowScrollOptions): UseFollowScrollReturn {
  const scroll_ref = useRef<HTMLDivElement>(null);
  const feed_ref = useRef<HTMLDivElement>(null);
  const bottom_anchor_ref = useRef<HTMLDivElement>(null);
  const should_follow_latest_ref = useRef(true);
  const last_scroll_top_ref = useRef(0);
  const pending_scroll_frame_ref = useRef<number | null>(null);
  const pending_scroll_inner_frame_ref = useRef<number | null>(null);
  const pending_prepend_restore_ref = useRef<{
    scroll_height: number;
    scroll_top: number;
  } | null>(null);
  const touch_start_y_ref = useRef<number | null>(null);
  const show_scroll_to_bottom_ref = useRef(false);
  const [show_scroll_to_bottom, setShowScrollToBottom] = useState(false);

  // ==================== 跟随状态 ====================

  const set_scroll_to_bottom_visibility = useCallback((visible: boolean) => {
    if (show_scroll_to_bottom_ref.current === visible) {
      return;
    }

    show_scroll_to_bottom_ref.current = visible;
    setShowScrollToBottom(visible);
  }, []);

  const update_follow_state = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) return;

    const distance_to_bottom =
      container.scrollHeight - container.scrollTop - container.clientHeight;
    const is_near_bottom = distance_to_bottom <= BOTTOM_THRESHOLD_PX;
    should_follow_latest_ref.current = is_near_bottom;
    set_scroll_to_bottom_visibility(!is_near_bottom);
  }, [set_scroll_to_bottom_visibility]);

  // ==================== 滚动调度 ====================

  const cancel_pending_scroll = useCallback(() => {
    if (pending_scroll_frame_ref.current !== null) {
      cancelAnimationFrame(pending_scroll_frame_ref.current);
      pending_scroll_frame_ref.current = null;
    }
    if (pending_scroll_inner_frame_ref.current !== null) {
      cancelAnimationFrame(pending_scroll_inner_frame_ref.current);
      pending_scroll_inner_frame_ref.current = null;
    }
  }, []);

  const schedule_scroll_to_bottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      cancel_pending_scroll();

      const container = scroll_ref.current;
      if (!container) return;

      // 流式输出时直接贴到底部，避免等待两帧后再修正位置导致换行抖动
      if (behavior === "auto") {
        container.scrollTop = container.scrollHeight;
        last_scroll_top_ref.current = container.scrollTop;
        return;
      }

      pending_scroll_frame_ref.current = requestAnimationFrame(() => {
        const next = scroll_ref.current;
        if (!next) return;
        const target_top = next.scrollHeight;
        const start_top = next.scrollTop;
        const distance = target_top - start_top;

        if (Math.abs(distance) < 1) {
          next.scrollTop = target_top;
          last_scroll_top_ref.current = next.scrollTop;
          return;
        }

        const start_time = performance.now();

        // 用固定时长动画替代浏览器默认 smooth，
        // 这样不同容器的滚动速度更一致，也更容易微调。
        const step = (now: number) => {
          const elapsed = now - start_time;
          const progress = Math.min(elapsed / SMOOTH_SCROLL_DURATION_MS, 1);
          const eased_progress = solve_bezier_progress(progress);

          next.scrollTop = start_top + distance * eased_progress;
          last_scroll_top_ref.current = next.scrollTop;

          if (progress < 1) {
            pending_scroll_inner_frame_ref.current =
              requestAnimationFrame(step);
          } else {
            pending_scroll_inner_frame_ref.current = null;
          }
        };

        pending_scroll_inner_frame_ref.current = requestAnimationFrame(step);
      });
    },
    [cancel_pending_scroll],
  );

  const scroll_to_bottom = useCallback(
    (behavior: ScrollBehavior = "smooth") => {
      should_follow_latest_ref.current = true;
      set_scroll_to_bottom_visibility(false);
      schedule_scroll_to_bottom(behavior);
    },
    [schedule_scroll_to_bottom, set_scroll_to_bottom_visibility],
  );

  const prepare_history_prepend_restore = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) {
      return;
    }
    cancel_pending_scroll();
    should_follow_latest_ref.current = false;
    pending_prepend_restore_ref.current = {
      scroll_height: container.scrollHeight,
      scroll_top: container.scrollTop,
    };
  }, [cancel_pending_scroll]);

  const cancel_history_prepend_restore = useCallback(() => {
    pending_prepend_restore_ref.current = null;
  }, []);

  // ==================== 副作用 ====================

  // 新消息 / loading 变化时自动滚动
  useLayoutEffect(() => {
    if (!should_follow_latest_ref.current) {
      // 用户主动离开底部后，仅保持按钮可见，避免流式消息期间重复触发同步 setState。
      set_scroll_to_bottom_visibility(true);
      return;
    }
    schedule_scroll_to_bottom(is_loading ? "auto" : "smooth");
  }, [
    auxiliary_block_count,
    is_loading,
    message_count,
    schedule_scroll_to_bottom,
    set_scroll_to_bottom_visibility,
  ]);

  useLayoutEffect(() => {
    const container = scroll_ref.current;
    const snapshot = pending_prepend_restore_ref.current;
    if (!container || !snapshot) {
      return;
    }
    pending_prepend_restore_ref.current = null;
    const height_delta = container.scrollHeight - snapshot.scroll_height;
    const next_scroll_top = snapshot.scroll_top + height_delta;
    container.scrollTop = next_scroll_top;
    last_scroll_top_ref.current = next_scroll_top;
    set_scroll_to_bottom_visibility(true);
  }, [history_prepend_token, set_scroll_to_bottom_visibility]);

  // feed 内容高度变化时保持跟随
  useEffect(() => {
    const feed = feed_ref.current;
    if (!feed || typeof ResizeObserver === "undefined") return;

    const observer = new ResizeObserver(() => {
      if (!should_follow_latest_ref.current) {
        set_scroll_to_bottom_visibility(true);
        return;
      }
      schedule_scroll_to_bottom("auto");
    });

    observer.observe(feed);
    return () => observer.disconnect();
  }, [schedule_scroll_to_bottom, set_scroll_to_bottom_visibility]);

  // session 切换时重置
  useEffect(() => {
    update_follow_state();
    last_scroll_top_ref.current = scroll_ref.current?.scrollTop || 0;
    pending_prepend_restore_ref.current = null;
  }, [update_follow_state, session_key]);

  // 卸载时清理
  useEffect(() => {
    return () => cancel_pending_scroll();
  }, [cancel_pending_scroll]);

  // ==================== 事件处理器 ====================

  const on_scroll = useCallback(() => {
    const container = scroll_ref.current;
    if (!container) return;

    const current_scroll_top = container.scrollTop;
    const is_scrolling_up = current_scroll_top < last_scroll_top_ref.current;
    last_scroll_top_ref.current = current_scroll_top;

    if (is_scrolling_up) cancel_pending_scroll();
    update_follow_state();
  }, [cancel_pending_scroll, update_follow_state]);

  const on_wheel = useCallback(
    (event: React.WheelEvent<HTMLDivElement>) => {
      if (event.deltaY < 0) cancel_pending_scroll();
    },
    [cancel_pending_scroll],
  );

  const on_touch_start = useCallback(
    (event: React.TouchEvent<HTMLDivElement>) => {
      touch_start_y_ref.current = event.touches[0]?.clientY ?? null;
    },
    [],
  );

  const on_touch_move = useCallback(
    (event: React.TouchEvent<HTMLDivElement>) => {
      const current_y = event.touches[0]?.clientY;
      if (current_y === undefined || touch_start_y_ref.current === null) return;
      if (current_y > touch_start_y_ref.current) cancel_pending_scroll();
    },
    [cancel_pending_scroll],
  );

  const on_touch_end = useCallback(() => {
    touch_start_y_ref.current = null;
  }, []);

  return {
    scroll_ref,
    feed_ref,
    bottom_anchor_ref,
    show_scroll_to_bottom,
    scroll_to_bottom,
    prepare_history_prepend_restore,
    cancel_history_prepend_restore,
    on_scroll,
    on_wheel,
    on_touch_start,
    on_touch_move,
    on_touch_end,
  };
}
