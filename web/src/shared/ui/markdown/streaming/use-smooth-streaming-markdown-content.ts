"use client";

/**
 * INPUT: runtime Markdown 快照、流式状态与仅 live 首帧可提供的从空显示标记。
 * OUTPUT: 单调追赶真实快照、终态继续排空 backlog 的平滑显示内容与渲染态。
 * POS: 统一 DM/Room 打字机节奏；首帧标记只影响 hook 初始化，历史/恢复正文立即显示。
 */
import { useCallback, useEffect, useRef, useState } from "react";

import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";

const STREAM_ACTIVE_INPUT_WINDOW_MS = 150;
const STREAM_TARGET_LAG_CHARS = 4;
const STREAM_ACTIVE_CPS = 84;
const STREAM_FLUSH_CPS = 300;
const STREAM_PRESSURE_THRESHOLD_CHARS = 48;
const STREAM_MAX_PRESSURE_REVEAL_CHARS = 10;

export interface SmoothStreamingMarkdownState {
  content: string;
  isStreaming: boolean;
}

function getNow(): number {
  return typeof performance === "undefined" ? Date.now() : performance.now();
}

function toChars(value: string): string[] {
  return Array.from(value);
}

function appendCharsInPlace(target: string[], value: string): number {
  let appendedCount = 0;
  for (const char of value) {
    target.push(char);
    appendedCount += 1;
  }
  return appendedCount;
}

export function resolveSmoothStreamingRevealCount({
  backlog,
  frameIntervalMs,
  inputActive,
}: {
  backlog: number;
  frameIntervalMs: number;
  inputActive: boolean;
}): number {
  const targetLagChars = inputActive ? STREAM_TARGET_LAG_CHARS : 0;
  const revealableBacklog = Math.max(0, backlog - targetLagChars);
  if (revealableBacklog === 0) {
    return 0;
  }

  const cps = inputActive ? STREAM_ACTIVE_CPS : STREAM_FLUSH_CPS;
  const timedReveal = Math.max(
    inputActive ? 1 : 2,
    Math.round((cps * Math.max(1, Math.min(frameIntervalMs, 50))) / 1000),
  );
  const pressureReveal = backlog > STREAM_PRESSURE_THRESHOLD_CHARS
    ? Math.min(
      STREAM_MAX_PRESSURE_REVEAL_CHARS,
      Math.ceil(backlog / 24),
    )
    : 0;

  return Math.min(
    revealableBacklog,
    Math.max(timedReveal, pressureReveal),
  );
}

export function useSmoothStreamingMarkdownState(
  content: string,
  enabled: boolean,
  initialRevealFromEmpty = false,
): SmoothStreamingMarkdownState {
  const prefersReducedMotion = usePrefersReducedMotion();
  const shouldInitializeEmpty = (
    initialRevealFromEmpty
    && !prefersReducedMotion
  );
  const initialDisplayedContent = shouldInitializeEmpty ? "" : content;
  const [displayedContent, setDisplayedContent] = useState(
    initialDisplayedContent,
  );
  const [isAnimating, setIsAnimatingState] = useState(
    shouldInitializeEmpty,
  );

  const targetInitialCharsRef = useRef<string[] | null>(null);
  if (targetInitialCharsRef.current === null) {
    targetInitialCharsRef.current = toChars(content);
  }
  const displayedContentRef = useRef(initialDisplayedContent);
  const displayedCountRef = useRef(
    shouldInitializeEmpty ? 0 : targetInitialCharsRef.current.length,
  );
  const targetContentRef = useRef(content);
  const targetCharsRef = useRef(targetInitialCharsRef.current);
  const targetCountRef = useRef(targetCharsRef.current.length);
  const lastInputTsRef = useRef(getNow());
  const lastFrameTsRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);
  const wakeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const enabledRef = useRef(enabled);
  const isAnimatingRef = useRef(shouldInitializeEmpty);

  const setIsAnimating = useCallback((next: boolean) => {
    if (isAnimatingRef.current === next) {
      return;
    }
    isAnimatingRef.current = next;
    setIsAnimatingState(next);
  }, []);

  const clearWakeTimer = useCallback(() => {
    if (wakeTimerRef.current !== null) {
      clearTimeout(wakeTimerRef.current);
      wakeTimerRef.current = null;
    }
  }, []);

  const stopFrameLoop = useCallback(() => {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    lastFrameTsRef.current = null;
  }, []);

  const stopScheduling = useCallback(() => {
    stopFrameLoop();
    clearWakeTimer();
  }, [clearWakeTimer, stopFrameLoop]);

  const startFrameLoopRef = useRef<() => void>(() => {});

  const scheduleWake = useCallback(
    (delayMs: number) => {
      clearWakeTimer();
      wakeTimerRef.current = setTimeout(() => {
        wakeTimerRef.current = null;
        startFrameLoopRef.current();
      }, Math.max(1, Math.ceil(delayMs)));
    },
    [clearWakeTimer],
  );

  const syncImmediate = useCallback(
    (nextContent: string) => {
      stopScheduling();

      const chars = toChars(nextContent);
      targetContentRef.current = nextContent;
      targetCharsRef.current = chars;
      targetCountRef.current = chars.length;
      displayedContentRef.current = nextContent;
      displayedCountRef.current = chars.length;
      lastInputTsRef.current = getNow();
      setIsAnimating(false);
      setDisplayedContent(nextContent);
    },
    [setIsAnimating, stopScheduling],
  );

  const startFrameLoop = useCallback(() => {
    clearWakeTimer();
    if (displayedCountRef.current >= targetCountRef.current) {
      setIsAnimating(false);
      return;
    }
    setIsAnimating(true);
    if (rafRef.current !== null) {
      return;
    }

    const tick = (timestamp: number) => {
      const previousFrameTs = lastFrameTsRef.current;
      const frameIntervalMs = previousFrameTs === null
        ? 16
        : Math.max(1, Math.min(timestamp - previousFrameTs, 50));
      lastFrameTsRef.current = timestamp;

      const targetCount = targetCountRef.current;
      const displayedCount = displayedCountRef.current;
      const backlog = targetCount - displayedCount;
      if (backlog <= 0) {
        stopFrameLoop();
        setIsAnimating(false);
        return;
      }

      const idleMs = getNow() - lastInputTsRef.current;
      const inputActive = (
        enabledRef.current
        && idleMs <= STREAM_ACTIVE_INPUT_WINDOW_MS
      );
      const revealCount = resolveSmoothStreamingRevealCount({
        backlog,
        frameIntervalMs,
        inputActive,
      });
      if (revealCount === 0) {
        stopFrameLoop();
        scheduleWake(STREAM_ACTIVE_INPUT_WINDOW_MS - idleMs + 8);
        return;
      }

      const nextCount = displayedCount + revealCount;
      const segment = targetCharsRef.current
        .slice(displayedCount, nextCount)
        .join("");
      const nextDisplayed = displayedContentRef.current + segment;

      displayedContentRef.current = nextDisplayed;
      displayedCountRef.current = nextCount;
      setDisplayedContent(nextDisplayed);

      rafRef.current = requestAnimationFrame(tick);
    };

    rafRef.current = requestAnimationFrame(tick);
  }, [
    clearWakeTimer,
    scheduleWake,
    setIsAnimating,
    stopFrameLoop,
  ]);

  startFrameLoopRef.current = startFrameLoop;

  useEffect(() => {
    const wasEnabled = enabledRef.current;
    enabledRef.current = enabled;
    if (prefersReducedMotion) {
      syncImmediate(content);
      return;
    }

    const previousTarget = targetContentRef.current;
    if (!enabled) {
      const canDrainTerminalAppend = (
        (wasEnabled || isAnimatingRef.current)
        && content.startsWith(previousTarget)
      );
      if (!canDrainTerminalAppend) {
        syncImmediate(content);
        return;
      }

      const appended = content.slice(previousTarget.length);
      if (appended) {
        targetContentRef.current = content;
        targetCountRef.current += appendCharsInPlace(
          targetCharsRef.current,
          appended,
        );
      }
      if (displayedCountRef.current < targetCountRef.current) {
        startFrameLoop();
      } else {
        stopScheduling();
        setIsAnimating(false);
      }
      return;
    }

    if (content === previousTarget) {
      if (displayedCountRef.current < targetCountRef.current) {
        startFrameLoop();
      }
      return;
    }

    const appended = content.startsWith(previousTarget)
      ? content.slice(previousTarget.length)
      : "";

    // 中文注释：历史回放、重载或运行时修正不是增量输入，必须立即对齐真实内容。
    if (!appended) {
      syncImmediate(content);
      return;
    }

    targetContentRef.current = content;
    targetCountRef.current += appendCharsInPlace(
      targetCharsRef.current,
      appended,
    );
    lastInputTsRef.current = getNow();
    startFrameLoop();
  }, [
    content,
    enabled,
    prefersReducedMotion,
    setIsAnimating,
    startFrameLoop,
    stopScheduling,
    syncImmediate,
  ]);

  useEffect(() => {
    return () => {
      stopScheduling();
    };
  }, [stopScheduling]);

  const willDrainTerminalAppend = (
    !enabled
    && enabledRef.current
    && content.startsWith(targetContentRef.current)
    && displayedContentRef.current !== content
  );
  const shouldRenderStreaming = (
    !prefersReducedMotion
    && (enabled || isAnimating || willDrainTerminalAppend)
  );
  return {
    content: shouldRenderStreaming ? displayedContent : content,
    isStreaming: shouldRenderStreaming,
  };
}

export function useSmoothStreamingMarkdownContent(
  content: string,
  enabled: boolean,
): string {
  return useSmoothStreamingMarkdownState(content, enabled).content;
}
