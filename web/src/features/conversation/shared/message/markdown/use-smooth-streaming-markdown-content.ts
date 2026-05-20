"use client";

import { useCallback, useEffect, useRef, useState } from "react";

const STREAM_ACTIVE_INPUT_WINDOW_MS = 170;
const STREAM_TARGET_LAG_CHARS = 5;
const STREAM_ACTIVE_CPS = 92;
const STREAM_FLUSH_CPS = 260;
const STREAM_LARGE_APPEND_CHARS = 220;

function get_now(): number {
  return typeof performance === "undefined" ? Date.now() : performance.now();
}

function count_chars(value: string): number {
  return [...value].length;
}

export function useSmoothStreamingMarkdownContent(content: string, enabled: boolean): string {
  const [displayed_content, set_displayed_content] = useState(content);

  const displayed_content_ref = useRef(content);
  const displayed_count_ref = useRef(count_chars(content));
  const target_content_ref = useRef(content);
  const target_chars_ref = useRef([...content]);
  const target_count_ref = useRef(target_chars_ref.current.length);
  const last_input_ts_ref = useRef(get_now());
  const last_frame_ts_ref = useRef<number | null>(null);
  const raf_ref = useRef<number | null>(null);
  const wake_timer_ref = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clear_wake_timer = useCallback(() => {
    if (wake_timer_ref.current !== null) {
      clearTimeout(wake_timer_ref.current);
      wake_timer_ref.current = null;
    }
  }, []);

  const stop_frame_loop = useCallback(() => {
    if (raf_ref.current !== null) {
      cancelAnimationFrame(raf_ref.current);
      raf_ref.current = null;
    }
    last_frame_ts_ref.current = null;
  }, []);

  const stop_scheduling = useCallback(() => {
    stop_frame_loop();
    clear_wake_timer();
  }, [clear_wake_timer, stop_frame_loop]);

  const start_frame_loop_ref = useRef<() => void>(() => {});

  const schedule_wake = useCallback(
    (delay_ms: number) => {
      clear_wake_timer();
      wake_timer_ref.current = setTimeout(() => {
        wake_timer_ref.current = null;
        start_frame_loop_ref.current();
      }, Math.max(1, Math.ceil(delay_ms)));
    },
    [clear_wake_timer],
  );

  const sync_immediate = useCallback(
    (next_content: string) => {
      stop_scheduling();

      const chars = [...next_content];
      target_content_ref.current = next_content;
      target_chars_ref.current = chars;
      target_count_ref.current = chars.length;
      displayed_content_ref.current = next_content;
      displayed_count_ref.current = chars.length;
      last_input_ts_ref.current = get_now();
      set_displayed_content(next_content);
    },
    [stop_scheduling],
  );

  const start_frame_loop = useCallback(() => {
    clear_wake_timer();
    if (raf_ref.current !== null) {
      return;
    }

    const tick = (timestamp: number) => {
      const previous_frame_ts = last_frame_ts_ref.current;
      const frame_interval_ms = previous_frame_ts === null
        ? 16
        : Math.max(1, Math.min(timestamp - previous_frame_ts, 50));
      last_frame_ts_ref.current = timestamp;

      const target_count = target_count_ref.current;
      const displayed_count = displayed_count_ref.current;
      const backlog = target_count - displayed_count;
      if (backlog <= 0) {
        stop_frame_loop();
        return;
      }

      const idle_ms = get_now() - last_input_ts_ref.current;
      const input_active = idle_ms <= STREAM_ACTIVE_INPUT_WINDOW_MS;
      const target_lag_chars = input_active ? STREAM_TARGET_LAG_CHARS : 0;
      const revealable_backlog = Math.max(0, backlog - target_lag_chars);
      if (revealable_backlog <= 0) {
        stop_frame_loop();
        schedule_wake(STREAM_ACTIVE_INPUT_WINDOW_MS - idle_ms + 8);
        return;
      }

      const cps = input_active ? STREAM_ACTIVE_CPS : STREAM_FLUSH_CPS;
      const timed_reveal = Math.max(
        input_active ? 1 : 2,
        Math.round((cps * frame_interval_ms) / 1000),
      );
      const pressure_reveal = backlog > 40 ? Math.ceil(backlog * 0.18) : 0;
      const reveal_count = Math.min(
        revealable_backlog,
        Math.max(timed_reveal, pressure_reveal),
      );
      const next_count = displayed_count + reveal_count;
      const segment = target_chars_ref.current.slice(displayed_count, next_count).join("");
      const next_displayed = displayed_content_ref.current + segment;

      displayed_content_ref.current = next_displayed;
      displayed_count_ref.current = next_count;
      set_displayed_content(next_displayed);

      raf_ref.current = requestAnimationFrame(tick);
    };

    raf_ref.current = requestAnimationFrame(tick);
  }, [clear_wake_timer, schedule_wake, stop_frame_loop]);

  start_frame_loop_ref.current = start_frame_loop;

  useEffect(() => {
    if (!enabled) {
      sync_immediate(content);
      return;
    }

    const previous_target = target_content_ref.current;
    if (content === previous_target) {
      return;
    }

    const appended = content.startsWith(previous_target)
      ? content.slice(previous_target.length)
      : "";
    const appended_count = count_chars(appended);

    // 中文注释：非追加更新通常来自历史回放、重载或运行时修正，必须立即对齐真实内容。
    if (!appended || appended_count > STREAM_LARGE_APPEND_CHARS) {
      sync_immediate(content);
      return;
    }

    target_content_ref.current = content;
    target_chars_ref.current = [...target_chars_ref.current, ...appended];
    target_count_ref.current += appended_count;
    last_input_ts_ref.current = get_now();
    start_frame_loop();
  }, [content, enabled, start_frame_loop, sync_immediate]);

  useEffect(() => {
    return () => {
      stop_scheduling();
    };
  }, [stop_scheduling]);

  return enabled ? displayed_content : content;
}
