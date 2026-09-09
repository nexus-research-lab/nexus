// INPUT: 桌面引导快照尚未返回时发生的用户重置。
// OUTPUT: 旧快照不能回写浏览器或桌面完成状态。
// POS: 引导持久化竞态回归。
import { expect, it, vi } from "vitest";
import { hydrateOnboardingStateFromDesktop, readCompletedTours, resetAllTourState } from "./tour-state";
const bridge = vi.hoisted(() => ({ get: vi.fn(), set: vi.fn(), remove: vi.fn(async () => {}) }));
vi.mock("@/lib/desktop-bridge", () => ({
  isDesktopBridgeAvailable: () => true,
  getDesktopPersistentState: bridge.get,
  setDesktopPersistentState: bridge.set,
  removeDesktopPersistentState: bridge.remove,
}));
it("does not persist a stale desktop snapshot after reset", async () => {
  let finish!: (value: { value: string }) => void;
  bridge.get.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  const hydration = hydrateOnboardingStateFromDesktop();
  resetAllTourState();
  finish({ value: '{"old-tour":true}' });
  expect((await hydration).completedTours).toEqual({});
  expect(readCompletedTours()).toEqual({});
  expect(bridge.set).not.toHaveBeenCalled();
});
