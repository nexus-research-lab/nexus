"use client";

/**
 * INPUT: runtime Markdown 快照、流式状态与仅 live 首帧可提供的从空显示标记。
 * OUTPUT: 单调追赶真实快照、终态继续排空 backlog 的平滑显示内容与渲染态。
 * POS: 统一 DM/Room 打字机节奏；首帧标记只影响 hook 初始化，历史/恢复正文立即显示。
 */
import {
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { AdaptiveStreamClock } from "./adaptive-stream-clock";

const LARGE_APPEND_CHARS = 500;

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
  const lastFrameTsRef = useRef<number | null>(null);
  const rafRef = useRef<number | null>(null);
  const enabledRef = useRef(enabled);
  const isAnimatingRef = useRef(shouldInitializeEmpty);
  const streamClockRef = useRef<AdaptiveStreamClock | null>(null);
  if (streamClockRef.current === null) {
    streamClockRef.current = new AdaptiveStreamClock(getNow());
  }

  const setIsAnimating = useCallback((next: boolean) => {
    if (isAnimatingRef.current === next) {
      return;
    }
    isAnimatingRef.current = next;
    setIsAnimatingState(next);
  }, []);

  const stopFrameLoop = useCallback(() => {
    if (rafRef.current !== null) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    lastFrameTsRef.current = null;
  }, []);

  const syncImmediate = useCallback(
    (nextContent: string) => {
      stopFrameLoop();

      const chars = toChars(nextContent);
      targetContentRef.current = nextContent;
      targetCharsRef.current = chars;
      targetCountRef.current = chars.length;
      displayedContentRef.current = nextContent;
      displayedCountRef.current = chars.length;
      streamClockRef.current?.reset(getNow());
      setIsAnimating(false);
      setDisplayedContent(nextContent);
    },
    [setIsAnimating, stopFrameLoop],
  );

  const startFrameLoop = useCallback(() => {
    if (displayedCountRef.current >= targetCountRef.current) {
      if (!enabledRef.current) {
        setIsAnimating(false);
      }
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
        : timestamp - previousFrameTs;
      lastFrameTsRef.current = timestamp;

      const targetCount = targetCountRef.current;
      const displayedCount = displayedCountRef.current;
      const backlog = targetCount - displayedCount;
      if (backlog <= 0) {
        if (enabledRef.current) {
          rafRef.current = requestAnimationFrame(tick);
        } else {
          stopFrameLoop();
          setIsAnimating(false);
        }
        return;
      }

      const frame = streamClockRef.current?.resolveFrame({
        backlog,
        frameIntervalMs,
        streaming: enabledRef.current,
        timestamp,
      });
      if (!frame) {
        stopFrameLoop();
        return;
      }
      if (frame.revealCount === 0) {
        rafRef.current = requestAnimationFrame(tick);
        return;
      }

      const nextCount = displayedCount + frame.revealCount;
      const segment = targetCharsRef.current
        .slice(displayedCount, nextCount)
        .join("");
      const nextDisplayed = displayedContentRef.current + segment;

      displayedContentRef.current = nextDisplayed;
      displayedCountRef.current = nextCount;
      setDisplayedContent(nextDisplayed);

      if (nextCount >= targetCountRef.current) {
        if (enabledRef.current) {
          rafRef.current = requestAnimationFrame(tick);
        } else {
          stopFrameLoop();
          setIsAnimating(false);
        }
        return;
      }

      rafRef.current = requestAnimationFrame(tick);
    };

    rafRef.current = requestAnimationFrame(tick);
  }, [
    setIsAnimating,
    stopFrameLoop,
  ]);

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
        if (toChars(appended).length > LARGE_APPEND_CHARS) {
          syncImmediate(content);
          return;
        }
        targetContentRef.current = content;
        const appendedCount = appendCharsInPlace(
          targetCharsRef.current,
          appended,
        );
        targetCountRef.current += appendedCount;
        streamClockRef.current?.observeAppend(getNow(), appendedCount);
      }
      if (displayedCountRef.current < targetCountRef.current) {
        startFrameLoop();
      } else {
        stopFrameLoop();
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
    if (toChars(appended).length > LARGE_APPEND_CHARS) {
      syncImmediate(content);
      return;
    }
    const appendedCount = appendCharsInPlace(
      targetCharsRef.current,
      appended,
    );
    targetCountRef.current += appendedCount;
    streamClockRef.current?.observeAppend(getNow(), appendedCount);
    startFrameLoop();
  }, [
    content,
    enabled,
    prefersReducedMotion,
    setIsAnimating,
    startFrameLoop,
    stopFrameLoop,
    syncImmediate,
  ]);

  useEffect(() => {
    return () => {
      stopFrameLoop();
    };
  }, [stopFrameLoop]);

  useEffect(() => {
    if (typeof document === "undefined") {
      return;
    }
    const handleVisibilityChange = () => {
      if (
        document.visibilityState === "visible"
        && displayedCountRef.current < targetCountRef.current
      ) {
        syncImmediate(targetContentRef.current);
      }
    };
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [syncImmediate]);

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
