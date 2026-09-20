// INPUT: Composer 人工介入高度保护与移动端 visualViewport resize 事件。
// OUTPUT: 软键盘连续 resize 只在稳定窗口后提交一次新的 intrinsic 高度。
// POS: Composer 几何稳定层的异步浏览器回归测试。
import { act, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { useComposerInteractionHeightGuard } from "./use-composer-interaction-height-guard";

type ResizeListener = () => void;

function createVisualViewportStub() {
  const listeners = new Set<ResizeListener>();
  const viewport = {
    addEventListener: (_type: string, listener: EventListenerOrEventListenerObject) => {
      listeners.add(listener as ResizeListener);
    },
    removeEventListener: (_type: string, listener: EventListenerOrEventListenerObject) => {
      listeners.delete(listener as ResizeListener);
    },
    emitResize: () => {
      for (const listener of listeners) listener();
    },
  };
  return viewport;
}

let measuredHeight = 0;

function Harness() {
  const elementRef = React.useRef<HTMLDivElement | null>(null);
  React.useLayoutEffect(() => {
    if (!elementRef.current) return;
    elementRef.current.getBoundingClientRect = () => ({
      bottom: measuredHeight,
      height: measuredHeight,
      left: 0,
      right: 320,
      top: 0,
      width: 320,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    } as DOMRect);
  }, []);
  useComposerInteractionHeightGuard({
    active: true,
    elementRef,
    scopeKey: "session",
  });
  return <div ref={elementRef} />;
}

describe("useComposerInteractionHeightGuard", () => {
  const originalVisualViewport = window.visualViewport;

  beforeEach(() => {
    vi.useFakeTimers();
    measuredHeight = 100;
  });

  afterEach(() => {
    vi.useRealTimers();
    Object.defineProperty(window, "visualViewport", {
      configurable: true,
      value: originalVisualViewport,
    });
  });

  it("coalesces keyboard resize bursts into one settled height commit", () => {
    const viewport = createVisualViewportStub();
    Object.defineProperty(window, "visualViewport", {
      configurable: true,
      value: viewport,
    });
    const view = render(<Harness />);
    const shell = view.container.firstElementChild as HTMLElement;
    expect(shell.style.minHeight).toBe("100px");

    measuredHeight = 180;
    act(() => {
      viewport.emitResize();
      vi.advanceTimersByTime(100);
      viewport.emitResize();
    });
    expect(shell.style.minHeight).toBe("100px");

    act(() => {
      vi.advanceTimersByTime(119);
    });
    expect(shell.style.minHeight).toBe("100px");

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(shell.style.minHeight).toBe("180px");
  });
});
