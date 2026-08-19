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
import { conversationStreamFrameScheduler } from "./stream-frame-scheduler";
import {
  appendStreamingTextUnits,
  joinStreamingTextPrefix,
  splitStreamingTextUnits,
} from "./stream-text-units";

export interface SmoothStreamingMarkdownState {
  content: string;
  isStreaming: boolean;
}

function getNow(): number {
  return typeof performance === "undefined" ? Date.now() : performance.now();
}

function toChars(value: string): string[] {
  return splitStreamingTextUnits(value);
}

function getRevealableTargetCount(
  targetCount: number,
  streaming: boolean,
): number {
  // live 尾 grapheme 仍可能被下一 transport delta 的 ZWJ/组合附标扩展；先保留
  // 一个单元，等后续单元确认边界或 runtime 终态后再展示。
  return streaming ? Math.max(0, targetCount - 1) : targetCount;
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
  const frameSubscriptionRef = useRef<(() => void) | null>(null);
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
    if (frameSubscriptionRef.current !== null) {
      frameSubscriptionRef.current();
      frameSubscriptionRef.current = null;
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

  const reconcileDisplayedTrailingUnit = useCallback(() => {
    const nextDisplayedContent = joinStreamingTextPrefix(
      targetCharsRef.current,
      displayedCountRef.current,
    );
    if (nextDisplayedContent === displayedContentRef.current) {
      return;
    }
    displayedContentRef.current = nextDisplayedContent;
    setDisplayedContent(nextDisplayedContent);
  }, []);

  const startFrameLoop = useCallback(() => {
    const revealableTargetCount = getRevealableTargetCount(
      targetCountRef.current,
      enabledRef.current,
    );
    if (displayedCountRef.current >= revealableTargetCount) {
      if (!enabledRef.current) {
        setIsAnimating(false);
      }
      return;
    }
    setIsAnimating(true);
    if (frameSubscriptionRef.current !== null) {
      return;
    }

    const tick = (timestamp: number, revealGrant: number): number => {
      const previousFrameTs = lastFrameTsRef.current;
      const frameIntervalMs = previousFrameTs === null
        ? 16
        : timestamp - previousFrameTs;
      lastFrameTsRef.current = timestamp;

      const targetCount = getRevealableTargetCount(
        targetCountRef.current,
        enabledRef.current,
      );
      const displayedCount = displayedCountRef.current;
      const backlog = targetCount - displayedCount;
      if (backlog <= 0) {
        stopFrameLoop();
        if (!enabledRef.current) {
          setIsAnimating(false);
        }
        return 0;
      }

      const frame = streamClockRef.current?.resolveFrame({
        backlog,
        frameIntervalMs,
        maxRevealCount: revealGrant,
        streaming: enabledRef.current,
        timestamp,
      });
      if (!frame) {
        stopFrameLoop();
        return 0;
      }
      if (frame.revealCount === 0) {
        return 0;
      }

      const nextCount = displayedCount + frame.revealCount;
      const segment = targetCharsRef.current
        .slice(displayedCount, nextCount)
        .join("");
      const nextDisplayed = displayedContentRef.current + segment;

      displayedContentRef.current = nextDisplayed;
      displayedCountRef.current = nextCount;
      setDisplayedContent(nextDisplayed);

      if (nextCount >= targetCount) {
        stopFrameLoop();
        if (!enabledRef.current) {
          setIsAnimating(false);
        }
      }
      return frame.revealCount;
    };

    frameSubscriptionRef.current = conversationStreamFrameScheduler.subscribe(
      tick,
    );
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
        targetContentRef.current = content;
        const previousTargetCount = targetCountRef.current;
        const appendResult = appendStreamingTextUnits(
          targetCharsRef.current,
          appended,
        );
        targetCountRef.current = targetCharsRef.current.length;
        if (
          appendResult.replacedTrailingUnit
          && displayedCountRef.current >= previousTargetCount
        ) {
          reconcileDisplayedTrailingUnit();
        }
        streamClockRef.current?.observeAppend(
          getNow(),
          appendResult.appendedCount,
        );
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
      const revealableTargetCount = getRevealableTargetCount(
        targetCountRef.current,
        true,
      );
      if (displayedCountRef.current < revealableTargetCount) {
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
    const previousTargetCount = targetCountRef.current;
    const appendResult = appendStreamingTextUnits(
      targetCharsRef.current,
      appended,
    );
    targetCountRef.current = targetCharsRef.current.length;
    if (
      appendResult.replacedTrailingUnit
      && displayedCountRef.current >= previousTargetCount
    ) {
      reconcileDisplayedTrailingUnit();
    }
    streamClockRef.current?.observeAppend(
      getNow(),
      appendResult.appendedCount,
    );
    startFrameLoop();
  }, [
    content,
    enabled,
    prefersReducedMotion,
    reconcileDisplayedTrailingUnit,
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
