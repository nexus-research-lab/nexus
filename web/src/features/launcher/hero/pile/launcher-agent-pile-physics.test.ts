// INPUT: 减少动效偏好、真实Matter场景和DOM引用。
// OUTPUT: 静态Token可见且没有定时掉落/动画帧，释放资源完整。
// POS: Launcher物理生命周期回归。
import { afterEach, expect, it, vi } from "vitest";
import type { SpotlightToken } from "@/types/app/launcher";
import { LauncherPilePhysics } from "./launcher-agent-pile-physics";
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });
it("settles a visible static scene without scheduling motion", () => {
  vi.stubGlobal("IntersectionObserver", class { observe() {} disconnect() {} });
  const raf = vi.spyOn(window, "requestAnimationFrame");
  const timer = vi.spyOn(window, "setTimeout");
  const container = document.createElement("div");
  Object.defineProperty(container, "clientHeight", { value: 286 });
  const element = document.createElement("button"); container.append(element);
  const token: SpotlightToken = { key: "a", label: "A", kind: "agent", agent_id: "a", swatch: { fill: "#123456", text: "#ffffff", ring: "#123456" } };
  const physics = new LauncherPilePhysics({
    container, reducedMotion: true,
    configs: [{ key: "a", angle: 0, delay: 50, radius: 20, size: 40, spawnX: 320, spawnY: -100 }],
    tokenByKey: new Map([["a", token]]), tokenRefs: { current: { a: element } },
  });
  expect(element.style.opacity).toBe("1");
  expect(element.style.transform).toContain("translate(");
  expect(raf).not.toHaveBeenCalled();
  expect(timer).not.toHaveBeenCalled();
  physics.dispose(); raf.mockRestore(); timer.mockRestore();
});
