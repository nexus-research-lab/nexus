import type { ConversationRoundScrollHandle } from "../../timeline/scroll/round-scroll";
import {
  findConversationRoundElement,
  getConversationRoundNavigationTarget,
  isConversationRoundScrollTargetVisible,
} from "../../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../../timeline/timeline-model";
import { scrollToTimelineRound } from "../navigation-dom";
import { isRoundLoaded, type PendingRoundJump } from "./round-jump-model";

const PENDING_SCROLL_MAX_RETRIES = 30;

type PendingRoundJumpLandingAttempt =
  | { status: "retry" }
  | { status: "waiting" }
  | { navigationRoundId: string; status: "landed" };

interface PendingRoundJumpLandingInput {
  roundScrollHandle: ConversationRoundScrollHandle | null;
  scrollElement: HTMLDivElement | null;
  target: PendingRoundJump;
  timeline: ConversationTimeline;
}

interface PendingRoundJumpLandingRuntimeOptions {
  attemptLanding: () => PendingRoundJumpLandingAttempt;
  onCancel: () => void;
  onLand: (navigationRoundId: string) => void;
}

interface AnimationFrameDriver {
  cancel: (frame: number) => void;
  request: (callback: FrameRequestCallback) => number;
}

const BROWSER_ANIMATION_FRAME_DRIVER: AnimationFrameDriver = {
  cancel: (frame) => window.cancelAnimationFrame(frame),
  request: (callback) => window.requestAnimationFrame(callback),
};

/** 持有单次导航落点的 RAF 与重试预算，不解释会话数据。 */
export class PendingRoundJumpLandingRuntime {
  private frame = 0;
  private retryCount = 0;

  constructor(
    private readonly options: PendingRoundJumpLandingRuntimeOptions,
    private readonly frameDriver: AnimationFrameDriver =
      BROWSER_ANIMATION_FRAME_DRIVER,
  ) {}

  start(): void {
    this.requestAttempt();
  }

  cancel(): void {
    if (!this.frame) {
      return;
    }
    this.frameDriver.cancel(this.frame);
    this.frame = 0;
  }

  private readonly attempt = (): void => {
    this.frame = 0;
    const result = this.options.attemptLanding();
    switch (result.status) {
      case "retry":
        this.scheduleRetry();
        return;
      case "waiting":
        return;
      case "landed":
        this.options.onLand(result.navigationRoundId);
    }
  };

  private requestAttempt(): void {
    this.frame = this.frameDriver.request(this.attempt);
  }

  private scheduleRetry(): void {
    if (this.retryCount >= PENDING_SCROLL_MAX_RETRIES) {
      this.options.onCancel();
      return;
    }
    this.retryCount += 1;
    this.requestAttempt();
  }
}

export function attemptPendingRoundJumpLanding({
  roundScrollHandle,
  scrollElement,
  target,
  timeline,
}: PendingRoundJumpLandingInput): PendingRoundJumpLandingAttempt {
  const loaded = isRoundLoaded(timeline, target.scrollRoundId);
  const didScroll = scrollToTimelineRound(
    scrollElement,
    roundScrollHandle,
    target.scrollRoundId,
    {
      align: "focus",
      behavior: loaded ? "auto" : "smooth",
    },
  );
  if (!didScroll) {
    return { status: "retry" };
  }
  if (!loaded) {
    return { status: "waiting" };
  }
  const navigationRoundId = resolveVisibleLandingRoundId(
    scrollElement,
    target,
  );
  return navigationRoundId
    ? { navigationRoundId, status: "landed" }
    : { status: "retry" };
}

function resolveVisibleLandingRoundId(
  scrollElement: HTMLDivElement | null,
  target: PendingRoundJump,
): string | null {
  if (!scrollElement) {
    return null;
  }
  const roundElement = findConversationRoundElement(
    scrollElement,
    target.scrollRoundId,
  );
  if (
    !roundElement
    || !isConversationRoundScrollTargetVisible(scrollElement, roundElement)
  ) {
    return null;
  }
  return getConversationRoundNavigationTarget(scrollElement)
    ?? target.navigationRoundId;
}
