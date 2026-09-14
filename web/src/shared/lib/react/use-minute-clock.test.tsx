// INPUT: Visible/hidden page lifecycle and enabled minute display consumers.
// OUTPUT: One aligned presentation clock, foreground refresh and complete timer/listener cleanup.
// POS: Offline lifecycle tests; the clock never owns business reads or task state.

import { act, renderHook } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useMinuteClock } from "./use-minute-clock";

const NOW = Date.UTC(2026, 8, 7, 12, 0, 15);
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); });

it("updates at minute boundaries and removes its timer when disabled or unmounted", () => {
  vi.useFakeTimers(); vi.setSystemTime(NOW);
  const { result, rerender, unmount } = renderHook(({ enabled }) => useMinuteClock(enabled), { initialProps: { enabled: false } });
  expect(result.current).toBe(NOW);
  expect(vi.getTimerCount()).toBe(0);
  rerender({ enabled: true });
  expect(vi.getTimerCount()).toBe(1);
  act(() => vi.advanceTimersByTime(44_999));
  expect(result.current).toBe(NOW);
  act(() => vi.advanceTimersByTime(1));
  expect(result.current).toBe(NOW + 45_000);
  act(() => vi.advanceTimersByTime(60_000));
  expect(result.current).toBe(NOW + 105_000);
  rerender({ enabled: false });
  expect(vi.getTimerCount()).toBe(0);
  rerender({ enabled: true });
  unmount();
  expect(vi.getTimerCount()).toBe(0);
  act(() => document.dispatchEvent(new Event("visibilitychange")));
  expect(vi.getTimerCount()).toBe(0);
});

it("suspends hidden pages and refreshes immediately on foreground without duplicate timers", () => {
  vi.useFakeTimers(); vi.setSystemTime(NOW);
  const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
  const { result, unmount } = renderHook(() => useMinuteClock(true));
  expect(vi.getTimerCount()).toBe(0);
  act(() => vi.advanceTimersByTime(300_000));
  expect(result.current).toBe(NOW);
  visibility.mockReturnValue("visible");
  act(() => { document.dispatchEvent(new Event("visibilitychange")); document.dispatchEvent(new Event("visibilitychange")); });
  expect(result.current).toBe(NOW + 300_000);
  expect(vi.getTimerCount()).toBe(1);
  visibility.mockReturnValue("hidden");
  act(() => document.dispatchEvent(new Event("visibilitychange")));
  expect(vi.getTimerCount()).toBe(0);
  unmount();
});
