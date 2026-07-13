import { getScrollBottomTop } from "./follow-scroll-model";

const SMOOTH_SCROLL_DURATION_MS = 420;
const EASING_CONTROL_POINTS = [0.23, 1, 0.32, 1] as const;

type ScrollContainerResolver = () => HTMLDivElement | null;
type ScrollPositionObserver = (scrollTop: number) => void;

export class BottomScrollAnimator {
  private scheduleFrameId: number | null = null;
  private animationFrameId: number | null = null;

  constructor(
    private readonly resolveContainer: ScrollContainerResolver,
    private readonly observePosition: ScrollPositionObserver,
  ) {}

  scroll(behavior: ScrollBehavior = "smooth"): void {
    this.cancel();
    const container = this.resolveContainer();
    if (!container) {
      return;
    }

    if (behavior === "auto") {
      this.setPosition(container, getScrollBottomTop(container));
      return;
    }

    this.scheduleFrameId = window.requestAnimationFrame(() => {
      this.scheduleFrameId = null;
      this.startSmoothScroll();
    });
  }

  cancel(): void {
    if (this.scheduleFrameId !== null) {
      window.cancelAnimationFrame(this.scheduleFrameId);
      this.scheduleFrameId = null;
    }
    if (this.animationFrameId !== null) {
      window.cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }
  }

  private startSmoothScroll(): void {
    const container = this.resolveContainer();
    if (!container) {
      return;
    }

    const targetTop = getScrollBottomTop(container);
    const startTop = container.scrollTop;
    const distance = targetTop - startTop;
    if (Math.abs(distance) < 1) {
      this.setPosition(container, targetTop);
      return;
    }

    const startTime = performance.now();
    const step = (now: number): void => {
      const progress = Math.min(
        (now - startTime) / SMOOTH_SCROLL_DURATION_MS,
        1,
      );
      this.setPosition(
        container,
        startTop + distance * solveBezierProgress(progress),
      );

      if (progress < 1) {
        this.animationFrameId = window.requestAnimationFrame(step);
        return;
      }
      this.animationFrameId = null;
    };

    this.animationFrameId = window.requestAnimationFrame(step);
  }

  private setPosition(container: HTMLDivElement, scrollTop: number): void {
    container.scrollTop = scrollTop;
    this.observePosition(container.scrollTop);
  }
}

function solveBezierProgress(progress: number): number {
  const [x1, y1, x2, y2] = EASING_CONTROL_POINTS;
  const clampedProgress = Math.min(Math.max(progress, 0), 1);
  const cx = 3 * x1;
  const bx = 3 * (x2 - x1) - cx;
  const ax = 1 - cx - bx;
  const cy = 3 * y1;
  const by = 3 * (y2 - y1) - cy;
  const ay = 1 - cy - by;

  let parameter = clampedProgress;
  for (let iteration = 0; iteration < 5; iteration += 1) {
    const error = sampleCubic(ax, bx, cx, parameter) - clampedProgress;
    const derivative = sampleCubicDerivative(ax, bx, cx, parameter);
    if (Math.abs(derivative) < 1e-6) {
      break;
    }
    parameter -= error / derivative;
  }

  let lower = 0;
  let upper = 1;
  parameter = Math.min(Math.max(parameter, 0), 1);
  for (let iteration = 0; iteration < 8; iteration += 1) {
    const sampledProgress = sampleCubic(ax, bx, cx, parameter);
    if (Math.abs(sampledProgress - clampedProgress) < 1e-5) {
      break;
    }
    if (sampledProgress > clampedProgress) {
      upper = parameter;
    } else {
      lower = parameter;
    }
    parameter = (lower + upper) / 2;
  }

  return sampleCubic(ay, by, cy, parameter);
}

function sampleCubic(a: number, b: number, c: number, value: number): number {
  return ((a * value + b) * value + c) * value;
}

function sampleCubicDerivative(
  a: number,
  b: number,
  c: number,
  value: number,
): number {
  return (3 * a * value + 2 * b) * value + c;
}
