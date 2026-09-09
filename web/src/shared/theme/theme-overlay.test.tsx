// INPUT: 主题、动态偏好及浏览器媒体/Canvas 可用性。
// OUTPUT: 验证不必要的动画不启动，媒体离开主题时停止。
// POS: 装饰生命周期回归，不做像素验收。
import { render } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { THEME_CONTEXT, type Theme } from "./theme-context";
import { ThemeOverlay } from "./theme-overlay";
const motion = vi.hoisted(() => ({ reduced: false }));
vi.mock("@/shared/lib/react/use-prefers-reduced-motion", () => ({ usePrefersReducedMotion: () => motion.reduced }));
afterEach(() => { motion.reduced = false; vi.restoreAllMocks(); });
function Scene({ theme }: { theme: Theme }) {
  return <THEME_CONTEXT.Provider value={{ theme, setTheme: vi.fn() }}><ThemeOverlay /></THEME_CONTEXT.Provider>;
}
it("does not start decorative media or canvas for reduced motion", () => {
  motion.reduced = true;
  const view = render(<Scene theme="sunny" />);
  expect(view.container.querySelector("video")).toBeNull();
  view.rerender(<Scene theme="rain" />);
  expect(view.container.querySelector("canvas")).toBeNull();
  expect((view.container.firstElementChild as HTMLElement).style.animation).toBe("none");
});
it("keeps a rain theme usable when a Canvas context is unavailable", () => {
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue(null);
  const raf = vi.spyOn(window, "requestAnimationFrame");
  const view = render(<Scene theme="rain" />);
  expect(view.container.querySelector("canvas")).not.toBeNull();
  expect(raf).not.toHaveBeenCalled();
});
it("stops the video when motion is reduced or the theme is left", () => {
  const play = vi.spyOn(HTMLMediaElement.prototype, "play").mockResolvedValue();
  const pause = vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  const view = render(<Scene theme="sunny" />);
  expect(play).toHaveBeenCalledOnce();
  motion.reduced = true;
  view.rerender(<Scene theme="sunny" />);
  expect(pause).toHaveBeenCalledOnce();
  expect(view.container.querySelector("video")).toBeNull();
  motion.reduced = false;
  view.rerender(<Scene theme="sunny" />);
  expect(play).toHaveBeenCalledTimes(2);
  view.rerender(<Scene theme="dark" />);
  expect(pause).toHaveBeenCalledTimes(2);
});
