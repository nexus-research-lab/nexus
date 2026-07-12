import type {
  ConnectorDeviceAuthPollResult,
  ConnectorDeviceAuthStart,
  ConnectorDeviceAuthStatus,
} from "@/types/capability/connector";

const DEFAULT_POLLING_MESSAGE = "等待 GitHub 授权确认";
const SLOW_DOWN_DELAY_MS = 5_000;

interface PollStatusRule {
  delayIncrementMs: number;
  fallbackMessage: string;
  outcome: "connected" | "failed" | "waiting";
}

const POLL_STATUS_RULES: Record<ConnectorDeviceAuthStatus, PollStatusRule> = {
  connected: {
    delayIncrementMs: 0,
    fallbackMessage: "GitHub 已授权",
    outcome: "connected",
  },
  denied: {
    delayIncrementMs: 0,
    fallbackMessage: "GitHub 授权未完成",
    outcome: "failed",
  },
  expired: {
    delayIncrementMs: 0,
    fallbackMessage: "GitHub 授权未完成",
    outcome: "failed",
  },
  pending: {
    delayIncrementMs: 0,
    fallbackMessage: DEFAULT_POLLING_MESSAGE,
    outcome: "waiting",
  },
  slow_down: {
    delayIncrementMs: SLOW_DOWN_DELAY_MS,
    fallbackMessage: DEFAULT_POLLING_MESSAGE,
    outcome: "waiting",
  },
};

export interface ConnectorDeviceAuthPollerCallbacks {
  onClose: () => void;
  onConnected: (connectorId: string) => Promise<void>;
  onError: (message: string) => void;
  onMessage: (message: string) => void;
}

type PollConnectorDeviceAuth = (
  connectorId: string,
  deviceCode: string,
) => Promise<ConnectorDeviceAuthPollResult>;

interface PollOutcome {
  delayIncrementMs: number;
  kind: PollStatusRule["outcome"];
  message: string;
}

function resolveConnectorDeviceAuthPollOutcome(
  result: ConnectorDeviceAuthPollResult,
): PollOutcome {
  const rule = POLL_STATUS_RULES[result.status];
  return {
    delayIncrementMs: rule.delayIncrementMs,
    kind: rule.outcome,
    message: result.message || rule.fallbackMessage,
  };
}

/** 轮询器独占定时器与终态，弹窗卸载后不会再发出回调。 */
export class ConnectorDeviceAuthPoller {
  private delayMs: number;
  private stopped = false;
  private timeoutId: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly session: ConnectorDeviceAuthStart,
    private readonly callbacks: ConnectorDeviceAuthPollerCallbacks,
    private readonly pollDeviceAuth: PollConnectorDeviceAuth,
  ) {
    this.delayMs = Math.max(session.interval || 5, 1) * 1_000;
  }

  start(): void {
    this.scheduleNextPoll();
  }

  stop(): void {
    this.stopped = true;
    if (this.timeoutId !== null) {
      clearTimeout(this.timeoutId);
      this.timeoutId = null;
    }
  }

  private scheduleNextPoll(): void {
    if (this.stopped) {
      return;
    }
    this.timeoutId = setTimeout(() => {
      this.timeoutId = null;
      void this.poll();
    }, this.delayMs);
  }

  private async poll(): Promise<void> {
    try {
      const result = await this.pollDeviceAuth(
        this.session.connector_id,
        this.session.device_code,
      );
      if (!this.stopped) {
        await this.handleOutcome(resolveConnectorDeviceAuthPollOutcome(result));
      }
    } catch (error) {
      this.fail(error instanceof Error ? error.message : "GitHub 授权轮询失败");
    }
  }

  private async handleOutcome(outcome: PollOutcome): Promise<void> {
    if (outcome.kind === "waiting") {
      this.delayMs += outcome.delayIncrementMs;
      this.callbacks.onMessage(outcome.message);
      this.scheduleNextPoll();
      return;
    }
    if (outcome.kind === "failed") {
      this.fail(outcome.message);
      return;
    }
    this.callbacks.onMessage(outcome.message);
    await this.callbacks.onConnected(this.session.connector_id);
    this.close();
  }

  private fail(message: string): void {
    if (this.stopped) {
      return;
    }
    try {
      this.callbacks.onError(message);
    } finally {
      this.close();
    }
  }

  private close(): void {
    if (this.stopped) {
      return;
    }
    this.stopped = true;
    this.callbacks.onClose();
  }
}
