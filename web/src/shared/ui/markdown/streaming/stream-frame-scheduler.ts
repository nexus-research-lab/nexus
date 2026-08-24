/**
 * INPUT: 当前仍有展示 backlog 的 Markdown 流订阅者与浏览器动画帧驱动。
 * OUTPUT: 全应用只保留一个 RAF，以固定总字符预算公平唤醒全部可见流。
 * POS: 多 Agent 流式正文的共享提交时钟；只调度展示内容，不预测或动画 DOM 高度。
 */

export type StreamFrameSubscriber = (
  timestamp: number,
  revealGrant: number,
) => number;

// 驱动最高 30Hz，且每个展示帧只允许一条流真正提交；长 Markdown 的同步
// 解析和布局不会因高刷新率屏幕或并行 Agent 越过这一上限。
const STREAM_PRESENTATION_FRAME_INTERVAL_MS = 1000 / 30;
const STREAM_PRESENTATION_FRAME_EPSILON_MS = 0.5;
// 无论同时存在多少个 Agent，每个展示帧最多只让真实 DOM 增长这一份额度。
// 4 个 grapheme 在 30Hz 下把全 Room 的展示上限控制在约 120/s；多个
// Agent 交错增长，避免同帧多张卡片一起跨行。
const STREAM_PRESENTATION_REVEAL_CAP = 4;

interface StreamFrameDriver {
  cancel: (frameId: number) => void;
  request: (callback: FrameRequestCallback) => number;
}

const browserFrameDriver: StreamFrameDriver = {
  cancel: (frameId) => window.cancelAnimationFrame(frameId),
  request: (callback) => window.requestAnimationFrame(callback),
};

export class StreamFrameScheduler {
  private frameId: number | null = null;
  private lastDispatchTimestamp: number | null = null;
  private nextSubscriber: StreamFrameSubscriber | null = null;
  private readonly subscribers = new Set<StreamFrameSubscriber>();

  constructor(
    private readonly driver: StreamFrameDriver = browserFrameDriver,
    private readonly revealCap = STREAM_PRESENTATION_REVEAL_CAP,
  ) {}

  subscribe(subscriber: StreamFrameSubscriber): () => void {
    this.subscribers.add(subscriber);
    if (this.nextSubscriber === null) {
      this.nextSubscriber = subscriber;
    }
    this.schedule();
    return () => {
      this.subscribers.delete(subscriber);
      if (this.subscribers.size === 0) {
        this.nextSubscriber = null;
        this.lastDispatchTimestamp = null;
        if (this.frameId !== null) {
          this.driver.cancel(this.frameId);
          this.frameId = null;
        }
      }
    };
  }

  private schedule(): void {
    if (this.frameId !== null || this.subscribers.size === 0) {
      return;
    }
    this.frameId = this.driver.request((timestamp) => {
      this.frameId = null;
      if (
        this.lastDispatchTimestamp === null
        || timestamp - this.lastDispatchTimestamp
          >= STREAM_PRESENTATION_FRAME_INTERVAL_MS
            - STREAM_PRESENTATION_FRAME_EPSILON_MS
      ) {
        this.lastDispatchTimestamp = timestamp;
        this.dispatch(timestamp);
      }
      this.schedule();
    });
  }

  private dispatch(timestamp: number): void {
    const currentSubscribers = [...this.subscribers];
    if (currentSubscribers.length === 0) {
      return;
    }

    const requestedStartIndex = this.nextSubscriber === null
      ? -1
      : currentSubscribers.indexOf(this.nextSubscriber);
    const startIndex = requestedStartIndex >= 0 ? requestedStartIndex : 0;
    const orderedSubscribers = [
      ...currentSubscribers.slice(startIndex),
      ...currentSubscribers.slice(0, startIndex),
    ];
    const revealGrant = Math.max(0, Math.floor(this.revealCap));
    if (revealGrant === 0) {
      return;
    }
    let firstVisitedIndex: number | null = null;

    for (let index = 0; index < orderedSubscribers.length; index += 1) {
      const subscriber = orderedSubscribers[index];
      if (!this.subscribers.has(subscriber)) {
        continue;
      }

      firstVisitedIndex ??= index;
      const consumed = subscriber(timestamp, revealGrant);
      if (
        !Number.isInteger(consumed)
        || consumed < 0
        || consumed > revealGrant
      ) {
        throw new Error(
          `Stream frame subscriber consumed ${consumed}; grant was ${revealGrant}`,
        );
      }
      if (consumed > 0) {
        this.nextSubscriber = this.findNextActiveSubscriber(
          orderedSubscribers,
          index,
        );
        return;
      }
    }

    // buffering 流返回 0，不占本帧唯一一次实际提交；若所有流都暂不可消费，
    // 下一帧仍轮转起点，避免首个 buffering 流长期占据探测顺序。
    this.nextSubscriber = this.findNextActiveSubscriber(
      orderedSubscribers,
      firstVisitedIndex ?? 0,
    );
  }

  private findNextActiveSubscriber(
    orderedSubscribers: StreamFrameSubscriber[],
    afterIndex: number,
  ): StreamFrameSubscriber | null {
    for (let offset = 1; offset <= orderedSubscribers.length; offset += 1) {
      const candidate = orderedSubscribers[
        (afterIndex + offset) % orderedSubscribers.length
      ];
      if (this.subscribers.has(candidate)) {
        return candidate;
      }
    }
    return [...this.subscribers][0] ?? null;
  }
}

export const conversationStreamFrameScheduler = new StreamFrameScheduler();
