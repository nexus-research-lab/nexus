// INPUT: Initial media preferences, changes, query replacement and missing browser support.
// OUTPUT: First-render preference accuracy and exact subscription cleanup.
// POS: Offline media subscription regressions shared by layout and reduced-motion consumers.

import { renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useSmoothStreamingMarkdownState } from "@/shared/ui/markdown/streaming/use-smooth-streaming-markdown-content";

afterEach(() => { vi.unstubAllGlobals(); });

function mediaQuery(initial: boolean) {
  let matches = initial;
  const listeners = new Set<() => void>();
  const media = {
    get matches() { return matches; },
    addEventListener: vi.fn((_event: string, listener: () => void) => { listeners.add(listener); }),
    removeEventListener: vi.fn((_event: string, listener: () => void) => { listeners.delete(listener); }),
  };
  return { media, listeners, change(value: boolean) { matches = value; listeners.forEach((listener) => listener()); } };
}

it("does not initially hide streaming Markdown when reduced motion is already enabled", () => {
  const query = mediaQuery(true);
  vi.stubGlobal("matchMedia", vi.fn(() => query.media));
  const renders: string[] = [];
  const { result } = renderHook(() => {
    const state = useSmoothStreamingMarkdownState("完整内容 👩🏽‍💻", true, true);
    renders.push(state.content);
    return state;
  });
  expect(renders[0]).toBe("完整内容 👩🏽‍💻");
  expect(result.current.content).toBe(renders[0]);
});
