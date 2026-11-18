/**
 * INPUT: 当前仍有展示 backlog 的 Markdown 流订阅者与浏览器动画帧驱动。
 * OUTPUT: 全应用只保留一个 RAF，以固定总字符预算与有界提交数公平唤醒全部可见流。
 * POS: 多 Agent 流式正文的共享提交时钟；只调度展示内容，不预测或动画 DOM 高度。
 */

export type StreamFrameSubscriber = (
  timestamp: number,
  revealGrant: number,
) => number;

// 驱动最高 30Hz。同一 RAF 内的多条流提交由 React 合成一次渲染与一次布局，
// FOLLOW 也只写一次 scrollTop；因此并行 Agent 同帧提交，避免每条流退化为 30/N Hz。
// 每帧最多提交 4 条流，长 Markdown 的同步解析不会随 Agent 数量无界增长。
const STREAM_PRESENTATION_FRAME_INTERVAL_MS = 1000 / 30;
const STREAM_PRESENTATION_MAX_COMMITS = 4;
const STREAM_PRESENTATION_FRAME_EPSILON_MS = 0.5;
// 字符时钟决定本帧需求；此上限约束同帧全部提交的总量，避免固定
// 4 字符把整个 Room 锁在 120 字/s。
const STREAM_PRESENTATION_REVEAL_CAP = 2048;

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
    let remainingGrant = Math.max(0, Math.floor(this.revealCap));
    if (remainingGrant === 0) {
      return;
    }
    let firstVisitedIndex: number | null = null;
    let lastCommittedIndex: number | null = null;
    let commits = 0;

    for (let index = 0; index < orderedSubscribers.length; index += 1) {
      const subscriber = orderedSubscribers[index];
      if (!this.subscribers.has(subscriber)) {
        continue;
      }

      firstVisitedIndex ??= index;
      const consumed = subscriber(timestamp, remainingGrant);
      if (
        !Number.isInteger(consumed)
        || consumed < 0
        || consumed > remainingGrant
      ) {
        throw new Error(
          `Stream frame subscriber consumed ${consumed}; grant was ${remainingGrant}`,
        );
      }
      if (consumed === 0) {
        continue;
      }
      lastCommittedIndex = index;
      remainingGrant -= consumed;
      commits += 1;
      if (remainingGrant === 0 || commits >= STREAM_PRESENTATION_MAX_COMMITS) {
        break;
      }
    }

    // 下一帧从最后一条实际提交之后开始；额度或提交数耗尽时未轮到的流优先。
    // buffering 流返回 0，不占提交名额；若全部暂不可消费，仍轮转探测起点。
    this.nextSubscriber = this.findNextActiveSubscriber(
      orderedSubscribers,
      lastCommittedIndex ?? firstVisitedIndex ?? 0,
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
