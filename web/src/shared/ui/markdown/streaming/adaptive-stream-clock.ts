/**
 * INPUT: runtime 追加节奏、当前缓冲字符数、渲染帧间隔与流式阶段。
 * OUTPUT: 先吸收传输抖动、再连续推进且在终态温和排空的字符预算。
 * POS: 与 React 和 Markdown 无关的流速时钟；传输停顿期间保持同一时间轴。
 */

const DEFAULT_ARRIVAL_CPS = 36;
const ARRIVAL_RATE_ALPHA = 0.18;
const ARRIVAL_WINDOW_MS = 3000;
const MIN_OBSERVED_CPS = 8;
const MAX_OBSERVED_CPS = 180;
const API_STALL_GAP_MS = 300;
const MIN_SAFE_GAP_MS = 500;
const MAX_SAFE_GAP_MS = 1200;
const MIN_INITIAL_BUFFER_CHARS = 12;
const MAX_INITIAL_BUFFER_CHARS = 28;
const INITIAL_BUFFER_SECONDS = 0.3;
const MAX_INITIAL_WAIT_MS = 600;
const MIN_LIVE_CPS = 18;
const MAX_LIVE_CPS = 90;
const LIVE_CATCH_UP_THRESHOLD_CHARS = 180;
const LIVE_MAX_BACKLOG_SECONDS = 5;
const MIN_FLUSH_CPS = 24;
const MAX_FLUSH_CPS = 120;
const FLUSH_SPEEDUP = 1.05;
const FLUSH_MAX_SECONDS = 8;

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, value));
}

function interpolate(current: number, next: number, alpha: number): number {
  return current * (1 - alpha) + next * alpha;
}

interface ArrivalSample {
  count: number;
  timestamp: number;
}

export type AdaptiveStreamPhase =
  | "buffering"
  | "flushing"
  | "rendering"
  | "waiting";

export interface AdaptiveStreamFrame {
  cps: number;
  phase: AdaptiveStreamPhase;
  revealCount: number;
}

interface ResolveFrameInput {
  backlog: number;
  frameIntervalMs: number;
  maxRevealCount?: number;
  streaming: boolean;
  timestamp: number;
}

export class AdaptiveStreamClock {
  private arrivalCps = DEFAULT_ARRIVAL_CPS;
  private readonly arrivals: ArrivalSample[] = [];
  private characterBudget = 0;
  private hasStartedRendering = false;
  private lastInputTimestamp: number | null = null;
  private maxGapMs = 0;
  private stallCount = 0;
  private streamStartTimestamp: number | null = null;
  private totalObservedCharacters = 0;

  constructor(timestamp: number) {
    this.reset(timestamp);
  }

  reset(_timestamp: number): void {
    this.arrivalCps = DEFAULT_ARRIVAL_CPS;
    this.arrivals.length = 0;
    this.characterBudget = 0;
    this.hasStartedRendering = false;
    this.lastInputTimestamp = null;
    this.maxGapMs = 0;
    this.stallCount = 0;
    this.streamStartTimestamp = null;
    this.totalObservedCharacters = 0;
  }

  observeAppend(timestamp: number, appendedCount: number): void {
    if (appendedCount <= 0) {
      return;
    }

    if (this.streamStartTimestamp === null) {
      this.streamStartTimestamp = timestamp;
    }
    if (this.lastInputTimestamp !== null) {
      const gapMs = Math.max(1, timestamp - this.lastInputTimestamp);
      this.maxGapMs = Math.max(this.maxGapMs, gapMs);
      if (gapMs > API_STALL_GAP_MS) {
        this.stallCount += 1;
      }
      const instantCps = clamp(
        (appendedCount * 1000) / gapMs,
        MIN_OBSERVED_CPS,
        MAX_OBSERVED_CPS,
      );
      this.arrivalCps = interpolate(
        this.arrivalCps,
        instantCps,
        ARRIVAL_RATE_ALPHA,
      );
    }

    this.lastInputTimestamp = timestamp;
    this.totalObservedCharacters += appendedCount;
    this.arrivals.push({ count: appendedCount, timestamp });
    this.pruneArrivals(timestamp);
  }

  resolveFrame({
    backlog,
    frameIntervalMs,
    maxRevealCount = Number.POSITIVE_INFINITY,
    streaming,
    timestamp,
  }: ResolveFrameInput): AdaptiveStreamFrame {
    if (backlog <= 0) {
      return { cps: 0, phase: "waiting", revealCount: 0 };
    }

    if (streaming && this.shouldBufferInitialContent(backlog, timestamp)) {
      this.characterBudget = 0;
      return { cps: 0, phase: "buffering", revealCount: 0 };
    }
    this.hasStartedRendering = true;

    const cps = streaming
      ? this.resolveLiveCps(backlog, timestamp)
      : this.resolveFlushCps(backlog, timestamp);
    // 这是当前流两次获得公平池探测机会之间的展示时间，不是 transport stall。
    // 四条以上 busy 流在 30Hz 下会自然等待 >100ms，必须完整累计；后台长停顿
    // 也安全，因为预算先被 backlog 封顶，实际提交再被全局 grant 限幅。
    const elapsedSeconds = Math.max(1, frameIntervalMs) / 1000;
    // 已经到达的 backlog 是当前唯一可消费的信用上限。多 Agent 公平池可能
    // 连续数帧只授予很小额度；若不封顶，旧 backlog 会积累成未来尚未到达
    // 字符的“预付预算”，在流恢复后长期以最高速率突发显示。
    this.characterBudget = Math.min(
      backlog,
      this.characterBudget + cps * elapsedSeconds,
    );
    const revealCount = Math.min(
      backlog,
      Math.floor(this.characterBudget),
      Math.max(0, Math.floor(maxRevealCount)),
    );
    // 只扣除调度器真正允许展示的字符；全局额度不足时，当前流已经累计的
    // 小数/整数预算继续留在时钟里，下一帧无需重新等待或损失原有语速。
    this.characterBudget -= revealCount;

    return {
      cps,
      phase: streaming ? "rendering" : "flushing",
      revealCount,
    };
  }

  private shouldBufferInitialContent(
    backlog: number,
    timestamp: number,
  ): boolean {
    if (this.hasStartedRendering || this.streamStartTimestamp === null) {
      return false;
    }
    const elapsedMs = timestamp - this.streamStartTimestamp;
    if (elapsedMs >= MAX_INITIAL_WAIT_MS) {
      return false;
    }
    const safeGapSeconds = Math.max(
      INITIAL_BUFFER_SECONDS,
      Math.min(this.maxGapMs, MAX_SAFE_GAP_MS) / 1000,
    );
    const targetBuffer = clamp(
      Math.round(this.arrivalCps * safeGapSeconds),
      MIN_INITIAL_BUFFER_CHARS,
      MAX_INITIAL_BUFFER_CHARS,
    );
    return backlog < targetBuffer;
  }

  private resolveLiveCps(backlog: number, timestamp: number): number {
    const recentArrivalCps = this.getRecentArrivalCps(timestamp);
    const elapsedSeconds = this.streamStartTimestamp === null
      ? 0
      : Math.max(0.001, (timestamp - this.streamStartTimestamp) / 1000);
    const effectiveArrivalCps = elapsedSeconds >= 0.5
      ? this.totalObservedCharacters / elapsedSeconds
      : recentArrivalCps;
    const safeGapSeconds = clamp(
      this.maxGapMs,
      MIN_SAFE_GAP_MS,
      MAX_SAFE_GAP_MS,
    ) / 1000;
    const safetyFactor = 1.45 + Math.min(this.stallCount, 4) * 0.15;
    const safeCps = backlog / (safeGapSeconds * safetyFactor);
    const arrivalCap = Math.min(
      recentArrivalCps,
      effectiveArrivalCps,
      this.arrivalCps,
    );
    const catchUpCps = backlog >= LIVE_CATCH_UP_THRESHOLD_CHARS
      ? backlog / LIVE_MAX_BACKLOG_SECONDS
      : 0;
    return clamp(
      Math.max(Math.min(safeCps, arrivalCap), catchUpCps),
      MIN_LIVE_CPS,
      MAX_LIVE_CPS,
    );
  }

  private resolveFlushCps(backlog: number, timestamp: number): number {
    const naturalCps = Math.max(
      MIN_FLUSH_CPS,
      this.getRecentArrivalCps(timestamp) * FLUSH_SPEEDUP,
    );
    const catchUpCps = backlog / FLUSH_MAX_SECONDS;
    return clamp(
      Math.max(naturalCps, catchUpCps),
      MIN_FLUSH_CPS,
      MAX_FLUSH_CPS,
    );
  }

  private getRecentArrivalCps(timestamp: number): number {
    this.pruneArrivals(timestamp);
    const first = this.arrivals[0];
    if (!first || this.arrivals.length < 2) {
      return this.arrivalCps;
    }
    const elapsedMs = Math.max(50, timestamp - first.timestamp);
    const characters = this.arrivals.reduce(
      (total, sample) => total + sample.count,
      0,
    );
    return clamp(
      (characters * 1000) / elapsedMs,
      MIN_OBSERVED_CPS,
      MAX_OBSERVED_CPS,
    );
  }

  private pruneArrivals(timestamp: number): void {
    const cutoff = timestamp - ARRIVAL_WINDOW_MS;
    while (
      this.arrivals.length > 1
      && this.arrivals[0].timestamp < cutoff
    ) {
      this.arrivals.shift();
    }
  }
}
