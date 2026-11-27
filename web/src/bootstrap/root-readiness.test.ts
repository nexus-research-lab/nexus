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
