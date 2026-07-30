/**
 * INPUT: 当前滚动容器、目标滚动行为与浏览器帧时序。
 * OUTPUT: paint 前同步 FOLLOW，或按实时底部阻尼移动的显式回到底部事务。
 * POS: 会话 FOLLOW 与用户触发平滑定位的唯一 scrollTop 写入执行器。
 */
import { getScrollBottomTop } from "./follow-scroll-model";

const SPRING_ANGULAR_FREQUENCY = 24;
const DEFAULT_FRAME_DELTA_SECONDS = 1 / 60;
const MAX_FRAME_DELTA_SECONDS = 1 / 15;
const SUSPENDED_FRAME_GAP_MS = 250;
const QUANTIZED_SCROLL_SNAP_DISTANCE_PX = 2;
const SETTLE_DISTANCE_PX = 0.5;
const REQUIRED_STABLE_FRAMES = 2;

type ScrollContainerResolver = () => HTMLDivElement | null;
type ScrollPositionObserver = (scrollTop: number) => void;
type ScrollAnimationMode = "scroll" | "settle";

export class BottomScrollAnimator {
  private animationFrameId: number | null = null;
  private animationMode: ScrollAnimationMode | null = null;
  private lastFrameTime: number | null = null;
  private stableFrameCount = 0;
  private velocity = 0;

  constructor(
    private readonly resolveContainer: ScrollContainerResolver,
    private readonly observePosition: ScrollPositionObserver,
  ) {}

  follow(): void {
    const container = this.resolveContainer();
    if (!container) {
      return;
    }

    // 用户点击“回到底部”后允许显式 smooth 事务完成；普通内容增长没有
    // 动画中间态，React layout effect / ResizeObserver 会在 paint 前贴底。
    if (this.animationMode === "scroll") {
      return;
    }
    if (this.animationMode === "settle") {
      this.cancel();
    }
    this.setPosition(container, getScrollBottomTop(container));
  }

  scroll(behavior: ScrollBehavior = "smooth"): void {
    this.cancel();
    const container = this.resolveContainer();
    if (!container) {
      return;
    }

    if (behavior === "auto" || prefersReducedMotion()) {
      this.setPosition(container, getScrollBottomTop(container));
      // 虚拟列表会在当前布局提交后才更新总高度；下一帧再次收口，
      // 避免先按旧 scrollHeight 贴底后被测量结果留在上方。
      this.animationMode = "settle";
      this.scheduleFrame();
      return;
    }

    this.startSpring();
  }

  cancel(): void {
    if (this.animationFrameId !== null) {
      window.cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }
    this.resetMotion();
  }

  private startSpring(): void {
    this.animationMode = "scroll";
    this.lastFrameTime = null;
    this.stableFrameCount = 0;
    this.velocity = 0;
    this.scheduleFrame();
  }

  private scheduleFrame(): void {
    if (this.animationFrameId !== null) {
      return;
    }
    this.animationFrameId = window.requestAnimationFrame((timestamp) => {
      this.animationFrameId = null;
      this.runFrame(timestamp);
    });
  }

  private runFrame(timestamp: number): void {
    const container = this.resolveContainer();
    const mode = this.animationMode;
    if (!container || !mode) {
      this.resetMotion();
      return;
    }

    if (mode === "settle") {
      this.setPosition(container, getScrollBottomTop(container));
      this.resetMotion();
      return;
    }

    const currentTop = container.scrollTop;
    const targetTop = getScrollBottomTop(container);
    const distance = targetTop - currentTop;
    if (Math.abs(distance) <= SETTLE_DISTANCE_PX) {
      this.setPosition(container, targetTop);
      this.velocity = 0;
      this.stableFrameCount += 1;
      if (this.stableFrameCount >= REQUIRED_STABLE_FRAMES) {
        this.resetMotion();
        return;
      }
      this.scheduleFrame();
      return;
    }

    const frameDeltaSeconds = resolveFrameDeltaSeconds(
      this.lastFrameTime,
      timestamp,
    );
    this.lastFrameTime = timestamp;
    let next = advanceCriticallyDampedSpring({
      current: currentTop,
      frameDeltaSeconds,
      target: targetTop,
      velocity: this.velocity,
    });
    if ((next.position - currentTop) * distance < 0) {
      // 内容收缩或视口变高时，旧的向下速度不能让滚动继续远离新目标。
      next = advanceCriticallyDampedSpring({
        current: currentTop,
        frameDeltaSeconds,
        target: targetTop,
        velocity: 0,
      });
    }
    if ((targetTop - next.position) * distance < 0) {
      next = { position: targetTop, velocity: 0 };
    }

    this.velocity = next.velocity;
    this.stableFrameCount = 0;
    const requestedPosition = next.position;
    this.setPosition(container, requestedPosition);
    if (
      container.scrollTop === currentTop
      && Math.abs(distance) <= QUANTIZED_SCROLL_SNAP_DISTANCE_PX
    ) {
      // Chromium/WKWebView 可能把 scrollTop 量化到 0.5/1px；小步进若被舍入，
      // 弹簧会永远停在目标前。此时直接收口，并保留一帧确认目标没有继续增长。
      this.setPosition(container, targetTop);
      this.velocity = 0;
      this.stableFrameCount = 1;
    }
    this.scheduleFrame();
  }

  private setPosition(container: HTMLDivElement, scrollTop: number): void {
    container.scrollTop = scrollTop;
    this.observePosition(container.scrollTop);
  }

  private resetMotion(): void {
    this.animationMode = null;
    this.lastFrameTime = null;
    this.stableFrameCount = 0;
    this.velocity = 0;
  }
}

interface SpringStepInput {
  current: number;
  frameDeltaSeconds: number;
  target: number;
  velocity: number;
}

function advanceCriticallyDampedSpring({
  current,
  frameDeltaSeconds,
  target,
  velocity,
}: SpringStepInput): { position: number; velocity: number } {
  const displacement = current - target;
  const spring = velocity + SPRING_ANGULAR_FREQUENCY * displacement;
  const decay = Math.exp(
    -SPRING_ANGULAR_FREQUENCY * frameDeltaSeconds,
  );
  return {
    position:
      target + (displacement + spring * frameDeltaSeconds) * decay,
    velocity:
      (
        velocity
        - SPRING_ANGULAR_FREQUENCY * spring * frameDeltaSeconds
      ) * decay,
  };
}

function resolveFrameDeltaSeconds(
  previousTimestamp: number | null,
  timestamp: number,
): number {
  if (previousTimestamp === null || timestamp <= previousTimestamp) {
    return DEFAULT_FRAME_DELTA_SECONDS;
  }
  const elapsedMs = timestamp - previousTimestamp;
  if (elapsedMs > SUSPENDED_FRAME_GAP_MS) {
    // 后台标签页和最小化 WebView 会暂停 rAF。恢复首帧只推进一个正常
    // frame，避免把隐藏期间积累的 Room 高度一次追赶成可见跳跃。
    return DEFAULT_FRAME_DELTA_SECONDS;
  }
  return Math.min(
    elapsedMs / 1_000,
    MAX_FRAME_DELTA_SECONDS,
  );
}

function prefersReducedMotion(): boolean {
  return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches
    ?? false;
}
