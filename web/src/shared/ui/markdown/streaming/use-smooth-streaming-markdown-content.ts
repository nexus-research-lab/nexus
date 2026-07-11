"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { findOpenMarkdownFenceLanguage } from "../core/markdown-fence";

const STREAM_ACTIVE_INPUT_WINDOW_MS = 170;
const STREAM_TARGET_LAG_CHARS = 5;
const STREAM_ACTIVE_CPS = 92;
const STREAM_FLUSH_CPS = 260;
const STREAM_LARGE_APPEND_CHARS = 220;

function getNow(): number {
  return typeof performance === "undefined" ? Date.now() : performance.now();
}

function countChars(value: string): number {
  return [...value].length;
}

function shouldBypassStreamBuffer(content: string): boolean {
  return findOpenMarkdownFenceLanguage(content) !== null;
}

export function useSmoothStreamingMarkdownContent(content: string, enabled: boolean): string {
  const [displayedContent, setDisplayedContent] = useState(content);

  const displayedContentRef = useRef(content);
  const displayedCountRef = useRef(countChars(content));
  const targetContentRef = useRef(content);
  const targetCharsRef = useRef([...content]);
  const targetCountRef = useRef(targetCharsRef.current.length);
  const lastInputTsRef = useRef(getNow());
  const lastFrameTsRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);
  const wakeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

      const chars = [...nextContent];
      targetContentRef.current = nextContent;
      targetCharsRef.current = chars;
      targetCountRef.current = chars.length;
      displayedContentRef.current = nextContent;
      displayedCountRef.current = chars.length;
      lastInputTsRef.current = getNow();
      setDisplayedContent(nextContent);
    },
    [stopScheduling],
  );

  const startFrameLoop = useCallback(() => {
    clearWakeTimer();
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
        return;
      }

      const idleMs = getNow() - lastInputTsRef.current;
      const inputActive = idleMs <= STREAM_ACTIVE_INPUT_WINDOW_MS;
      const targetLagChars = inputActive ? STREAM_TARGET_LAG_CHARS : 0;
      const revealableBacklog = Math.max(0, backlog - targetLagChars);
      if (revealableBacklog <= 0) {
        stopFrameLoop();
        scheduleWake(STREAM_ACTIVE_INPUT_WINDOW_MS - idleMs + 8);
        return;
      }

      const cps = inputActive ? STREAM_ACTIVE_CPS : STREAM_FLUSH_CPS;
      const timedReveal = Math.max(
        inputActive ? 1 : 2,
        Math.round((cps * frameIntervalMs) / 1000),
      );
      const pressureReveal = backlog > 40 ? Math.ceil(backlog * 0.18) : 0;
      const revealCount = Math.min(
        revealableBacklog,
        Math.max(timedReveal, pressureReveal),
      );
      const nextCount = displayedCount + revealCount;
      const segment = targetCharsRef.current.slice(displayedCount, nextCount).join("");
      const nextDisplayed = displayedContentRef.current + segment;

      displayedContentRef.current = nextDisplayed;
      displayedCountRef.current = nextCount;
      setDisplayedContent(nextDisplayed);

      rafRef.current = requestAnimationFrame(tick);
    };

    rafRef.current = requestAnimationFrame(tick);
  }, [clearWakeTimer, scheduleWake, stopFrameLoop]);

  startFrameLoopRef.current = startFrameLoop;

  useEffect(() => {
    if (!enabled) {
      syncImmediate(content);
      return;
    }

    const previousTarget = targetContentRef.current;
    if (content === previousTarget) {
      return;
    }

    const appended = content.startsWith(previousTarget)
      ? content.slice(previousTarget.length)
      : "";
    const appendedCount = countChars(appended);

    // 中文注释：非追加更新通常来自历史回放、重载或运行时修正，必须立即对齐真实内容。
    if (
      !appended ||
      appendedCount > STREAM_LARGE_APPEND_CHARS ||
      shouldBypassStreamBuffer(content)
    ) {
      syncImmediate(content);
      return;
    }

    targetContentRef.current = content;
    targetCharsRef.current = [...targetCharsRef.current, ...appended];
    targetCountRef.current += appendedCount;
    lastInputTsRef.current = getNow();
    startFrameLoop();
  }, [content, enabled, startFrameLoop, syncImmediate]);

  useEffect(() => {
    return () => {
      stopScheduling();
    };
  }, [stopScheduling]);

  return enabled ? displayedContent : content;
}
