import type { FailureCore } from "@/types/generated/protocol";

export type ApiTransportFailureKind =
  | "network"
  | "response_interrupted"
  | "timeout";

export type ApiTransportFailureEffect = "not_applicable" | "unknown";

/**
 * 没有拿到完整 HTTP 响应时的本地传输事实。
 *
 * 它不是服务端 FailureCore，也不提供重试动作；写请求只能保守标记为 unknown。
 */
export class ApiTransportError extends Error {
  readonly category: "timeout" | "unavailable";
  readonly effect: ApiTransportFailureEffect;
  readonly kind: ApiTransportFailureKind;
  readonly status: number | null;
  readonly transportRequestId: string | null;

  constructor(
    message: string,
    kind: ApiTransportFailureKind,
    effect: ApiTransportFailureEffect,
    options?: {
      status?: number | null;
      transportRequestId?: string | null;
    },
  ) {
    super(message);
    this.name = "ApiTransportError";
    this.category = kind === "timeout" ? "timeout" : "unavailable";
    this.effect = effect;
    this.kind = kind;
    this.status = options?.status ?? null;
    this.transportRequestId = options?.transportRequestId?.trim() || null;
  }
}

export class UnauthorizedError extends Error {
  readonly failure: FailureCore | null;
  readonly status = 401;
  readonly transportRequestId: string | null;

  constructor(
    message = "未登录或登录状态已过期",
    failure: FailureCore | null = null,
  ) {
    super(message);
    this.name = "UnauthorizedError";
    this.failure = failure;
    this.transportRequestId = failure?.transport_request_id ?? null;
  }
}

export class ApiRequestError extends Error {
  readonly failure: FailureCore | null;
  readonly status: number;
  readonly transportRequestId: string | null;

  constructor(
    message: string,
    status: number,
    failure: FailureCore | null = null,
  ) {
    super(message);
    this.name = "ApiRequestError";
    this.failure = failure;
    this.status = status;
    this.transportRequestId = failure?.transport_request_id ?? null;
  }
}
