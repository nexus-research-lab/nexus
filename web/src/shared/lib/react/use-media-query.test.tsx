// INPUT: Initial media preferences, changes, query replacement and missing browser support.
// OUTPUT: First-render preference accuracy and exact subscription cleanup.
// POS: Offline media subscription regressions shared by layout and reduced-motion consumers.

import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useMediaQuery } from "./use-media-query";
import { usePrefersReducedMotion } from "./use-prefers-reduced-motion";
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

it("reads reduced motion on the first render and follows later preference changes", () => {
  const query = mediaQuery(true);
  const matchMedia = vi.fn(() => query.media);
  vi.stubGlobal("matchMedia", matchMedia);
  const renders: boolean[] = [];
  const { result, unmount } = renderHook(() => {
    const reduced = usePrefersReducedMotion();
    renders.push(reduced);
    return reduced;
  });
  expect(renders[0]).toBe(true);
  expect(matchMedia).toHaveBeenCalledWith("(prefers-reduced-motion: reduce)");
  expect(query.listeners.size).toBe(1);
  act(() => query.change(false));
  expect(result.current).toBe(false);
  act(() => query.change(true));
  expect(result.current).toBe(true);
  unmount();
  expect(query.listeners.size).toBe(0);
});

it("replaces the exact query subscription without letting old events overwrite it", () => {
  const first = mediaQuery(true);
  const second = mediaQuery(false);
  vi.stubGlobal("matchMedia", vi.fn((query: string) => query === "first" ? first.media : second.media));
  const { result, rerender, unmount } = renderHook(({ query }) => useMediaQuery(query), { initialProps: { query: "first" } });
  expect(result.current).toBe(true);
  rerender({ query: "second" });
  expect(result.current).toBe(false);
  expect(first.listeners.size).toBe(0);
  expect(second.listeners.size).toBe(1);
  act(() => first.change(true));
  expect(result.current).toBe(false);
  act(() => second.change(true));
  expect(result.current).toBe(true);
  unmount();
  expect(second.listeners.size).toBe(0);
});

it("keeps the default preference when matchMedia is unavailable", () => {
  vi.stubGlobal("matchMedia", undefined);
  const { result } = renderHook(() => usePrefersReducedMotion());
  expect(result.current).toBe(false);
});

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
