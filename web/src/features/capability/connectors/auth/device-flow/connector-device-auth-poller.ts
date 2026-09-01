// INPUT: 单次 Device Flow 会话、provider 轮询结果与网络失败。
// OUTPUT: 明确区分“确认未连接”和“结果未知”的终态回调；不执行配置清理或写入重放。
// POS: Connector Device Flow 的纯轮询状态机，资源恢复决定留给上层控制器。
import type {
  ConnectorDeviceAuthPollResult,
  ConnectorDeviceAuthStart,
  ConnectorDeviceAuthStatus,
} from "@/types/capability/connector";
import { getErrorMessage } from "@/lib/error-message";

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
  onError: (
    message: string,
    kind: ConnectorDeviceAuthFailureKind,
  ) => void;
  onMessage: (message: string) => void;
  onNext: (session: ConnectorDeviceAuthStart) => void;
}

export type ConnectorDeviceAuthFailureKind =
  | "not_connected"
  | "outcome_unknown";

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
        if (result.next) {
          this.callbacks.onMessage(
            result.message || "应用已选择或创建，请继续完成账号授权",
          );
          this.stop();
          this.callbacks.onNext(result.next);
          return;
        }
        await this.handleOutcome(resolveConnectorDeviceAuthPollOutcome(result));
      }
    } catch (error) {
      this.fail(
        getErrorMessage(
          error,
          this.session.connector_id === "feishu-docx"
            ? "飞书授权状态暂时无法确认"
            : "GitHub 授权状态暂时无法确认",
        ),
        "outcome_unknown",
      );
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
      this.fail(outcome.message, "not_connected");
      return;
    }
    this.callbacks.onMessage(outcome.message);
    this.close();
    await this.callbacks.onConnected(this.session.connector_id);
  }

  private fail(
    message: string,
    kind: ConnectorDeviceAuthFailureKind,
  ): void {
    if (this.stopped) {
      return;
    }
    try {
      this.callbacks.onError(message, kind);
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
