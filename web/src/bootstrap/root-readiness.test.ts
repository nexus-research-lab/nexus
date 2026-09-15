// INPUT: Root commits, delayed route/auth placeholders, and browser paint timers.
// OUTPUT: Readiness cannot overtake lazy loading or survive cancellation.
// POS: Desktop bootstrap readiness regressions without a native host.
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { observeRootReadiness } from "./root-readiness";

let root: HTMLElement;
let stop: (() => void) | undefined;
beforeEach(() => {
  vi.useFakeTimers();
  root = document.createElement("div");
  document.body.append(root);
});
afterEach(() => {
  stop?.();
  root.remove();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it("waits through an empty root and delayed route loading before reporting once", async () => {
  const ready = vi.fn();
  stop = observeRootReadiness(root, ready);
  await vi.advanceTimersByTimeAsync(500);
  expect(ready).not.toHaveBeenCalled();
  root.innerHTML = '<main data-bootstrap-pending="true">Loading</main>';
  await vi.advanceTimersByTimeAsync(500);
  expect(ready).not.toHaveBeenCalled();
  root.innerHTML = "<main>Launcher</main>";
  await vi.advanceTimersByTimeAsync(300);
  expect(ready).toHaveBeenCalledTimes(1);
  root.innerHTML = "<main>Updated</main>";
  await vi.advanceTimersByTimeAsync(300);
  expect(ready).toHaveBeenCalledTimes(1);
});

it("rechecks loading introduced between a commit and its scheduled paint", async () => {
  root.innerHTML = "<main>Shell</main>";
  const ready = vi.fn();
  stop = observeRootReadiness(root, ready);
  root.firstElementChild!.setAttribute("data-bootstrap-pending", "true");
  await vi.advanceTimersByTimeAsync(500);
  expect(ready).not.toHaveBeenCalled();
  root.firstElementChild!.removeAttribute("data-bootstrap-pending");
  await vi.advanceTimersByTimeAsync(300);
  expect(ready).toHaveBeenCalledTimes(1);
});

it("allows a committed error page and cancels pending notifications", async () => {
  root.innerHTML = "<main>Unable to load. Retry</main>";
  const ready = vi.fn();
  stop = observeRootReadiness(root, ready);
  stop();
  await vi.advanceTimersByTimeAsync(500);
  expect(ready).not.toHaveBeenCalled();
  stop = observeRootReadiness(root, ready);
  await vi.advanceTimersByTimeAsync(300);
  expect(ready).toHaveBeenCalledTimes(1);
});

it("uses the background timer only after loading has resolved", async () => {
  vi.spyOn(window, "requestAnimationFrame").mockReturnValue(1);
  root.innerHTML = '<main data-bootstrap-pending="true">Loading</main>';
  const ready = vi.fn();
  stop = observeRootReadiness(root, ready);
  await vi.advanceTimersByTimeAsync(500);
  expect(ready).not.toHaveBeenCalled();
  root.innerHTML = "<main>Launcher</main>";
  await vi.advanceTimersByTimeAsync(300);
  expect(ready).toHaveBeenCalledExactlyOnceWith("timerFallback");
});
