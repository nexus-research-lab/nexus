/**
 * INPUT: 当前滚动容器、目标滚动行为与浏览器帧时序。
 * OUTPUT: 可取消、底部目标单向增长的阻尼跟随，或按实时目标二阶段收口的显式贴底。
 * POS: 会话滚动写入的唯一动态跟随与二阶段贴底执行器。
 */
import {
  getConversationViewportSize,
  getScrollBottomTop,
  hasConversationViewportSizeChanged,
  type ConversationViewportSize,
} from "./follow-scroll-model";

const SPRING_ANGULAR_FREQUENCY = 24;
const DEFAULT_FRAME_DELTA_SECONDS = 1 / 60;
const MAX_FRAME_DELTA_SECONDS = 1 / 15;
const SUSPENDED_FRAME_GAP_MS = 250;
const QUANTIZED_SCROLL_SNAP_DISTANCE_PX = 2;
const SETTLE_DISTANCE_PX = 0.5;
const REQUIRED_STABLE_FRAMES = 2;

type ScrollContainerResolver = () => HTMLDivElement | null;
type ScrollPositionObserver = (scrollTop: number) => void;
type ScrollAnimationMode = "follow" | "scroll" | "settle";

export class BottomScrollAnimator {
  private animationFrameId: number | null = null;
  private animationMode: ScrollAnimationMode | null = null;
  private followTargetTop: number | null = null;
  private followViewportSize: ConversationViewportSize | null = null;
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
    if (prefersReducedMotion()) {
      this.cancel();
      this.setPosition(
        container,
        Math.max(container.scrollTop, getScrollBottomTop(container)),
      );
      return;
    }

    if (
      this.animationMode === "follow"
      || this.animationMode === "scroll"
    ) {
      return;
    }
    if (this.animationMode === "settle") {
      this.cancel();
    }
    this.stableFrameCount = 0;
    this.startSpring("follow");
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

    this.startSpring("scroll");
  }

  cancel(): void {
    if (this.animationFrameId !== null) {
      window.cancelAnimationFrame(this.animationFrameId);
      this.animationFrameId = null;
    }
    this.resetMotion();
  }

  private startSpring(mode: "follow" | "scroll"): void {
    const container = this.resolveContainer();
    this.animationMode = mode;
    this.followTargetTop = null;
    this.followViewportSize = mode === "follow" && container
      ? getConversationViewportSize(container)
      : null;
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
    if (
      mode === "follow"
      && this.followViewportSize
      && hasConversationViewportSizeChanged(
        this.followViewportSize,
        getConversationViewportSize(container),
      )
    ) {
      // Composer、虚拟键盘或 App 窗口改变的是阅读视口，不是模型正文。
      // 在写入新位置前停止，外层 ResizeObserver 会同步切到 detached。
      this.resetMotion();
      return;
    }

    const currentTop = container.scrollTop;
    const measuredTargetTop = getScrollBottomTop(container);
    const targetTop = mode === "follow"
      ? this.resolveFollowTarget(measuredTargetTop, currentTop)
      : measuredTargetTop;
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
      mode === "follow"
      && container.scrollTop + SETTLE_DISTANCE_PX < requestedPosition
      && getScrollBottomTop(container)
        <= container.scrollTop + SETTLE_DISTANCE_PX
    ) {
      // 永久高度收缩时浏览器会把写入值钳制在真实 bottom。此时结束旧的
      // 高水位事务；后续内容再增长会由 ResizeObserver 启动新的单向跟随。
      this.resetMotion();
      return;
    }
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

  private resolveFollowTarget(
    measuredTargetTop: number,
    currentTop: number,
  ): number {
    // ResizeObserver 和虚拟列表测高会在同一次布局提交中短暂报告较小的
    // scrollHeight。自动跟随只接受底部目标单向增长，避免先主动向上、
    // 下一帧又向下；显式 scroll() 使用实时目标，不受这个高水位约束。
    this.followTargetTop = Math.max(
      this.followTargetTop ?? currentTop,
      measuredTargetTop,
      currentTop,
    );
    return this.followTargetTop;
  }

  private resetMotion(): void {
    this.animationMode = null;
    this.followTargetTop = null;
    this.followViewportSize = null;
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
