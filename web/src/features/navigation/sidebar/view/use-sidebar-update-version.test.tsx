// INPUT: Deferred bridge reads and polling time.
// OUTPUT: Serial updates and disposal of late responses.
// POS: Sidebar version hint hook regression.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useSidebarUpdateVersion } from "./use-sidebar-update-version";
const mocks = vi.hoisted(() => ({ desktop: vi.fn(), available: vi.fn(), read: vi.fn() }));
vi.mock("@/config/desktop-runtime", () => ({ isDesktopRuntime: mocks.desktop }));
vi.mock("@/lib/desktop-bridge", () => ({ isDesktopBridgeAvailable: mocks.available, getDesktopPersistentState: mocks.read }));
beforeEach(() => {
  vi.useFakeTimers(); mocks.desktop.mockReturnValue(true); mocks.available.mockReturnValue(true); mocks.read.mockReset();
});
afterEach(() => { vi.useRealTimers(); });
it.each([[false, true], [true, false]])("does not poll without desktop bridge (%s, %s)", async (desktop, available) => {
  mocks.desktop.mockReturnValue(desktop); mocks.available.mockReturnValue(available);
  const hook = renderHook(useSidebarUpdateVersion);
  await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
  expect(mocks.read).not.toHaveBeenCalled(); expect(hook.result.current).toBeNull();
});
it("skips overlapping reads and retains a known version on failure", async () => {
  let resolve!: (value: { value: string }) => void;
  mocks.read.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  const hook = renderHook(useSidebarUpdateVersion);
  await act(async () => { await vi.advanceTimersByTimeAsync(90_000); });
  expect(mocks.read).toHaveBeenCalledTimes(1);
  expect(mocks.read).toHaveBeenCalledWith("desktop.update.available");
  await act(async () => { resolve({ value: " 1.2.3 " }); });
  expect(hook.result.current).toBe("1.2.3");
  mocks.read.mockRejectedValueOnce(new Error("unavailable"));
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(hook.result.current).toBe("1.2.3");
  mocks.read.mockResolvedValueOnce({ value: " " });
  await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
  expect(hook.result.current).toBeNull(); expect(mocks.read).toHaveBeenCalledTimes(3);
});
it("discards old mount responses and stops polling on unmount", async () => {
  let resolve!: (value: { value: string }) => void;
  mocks.read.mockImplementationOnce(() => new Promise((done) => { resolve = done; }));
  const old = renderHook(useSidebarUpdateVersion); old.unmount();
  mocks.read.mockResolvedValue({ value: "new" });
  const current = renderHook(useSidebarUpdateVersion);
  await act(async () => { resolve({ value: "old" }); });
  expect(current.result.current).toBe("new"); current.unmount();
  await act(async () => { await vi.advanceTimersByTimeAsync(90_000); });
  expect(mocks.read).toHaveBeenCalledTimes(2);
});
