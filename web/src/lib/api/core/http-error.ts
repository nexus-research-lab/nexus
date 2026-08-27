import type { FailureCore } from "@/types/generated/protocol";

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
