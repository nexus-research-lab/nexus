// INPUT: 任意失败值、服务端 FailureCore、本地传输事实与当前操作的安全兜底文案。
// OUTPUT: 资源访问失效和修改结果证据的纯投影；不解释或执行恢复动作。
// POS: Web 错误语义的最小边界；不根据文案或浏览器网络提示猜测业务结果。
import {
  ApiRequestError,
  ApiTransportError,
  UnauthorizedError,
} from "@/lib/api/core/http-error";

const INTERNAL_ERROR_PLACEHOLDERS = new Set([
  "服务内部错误",
  "内部服务错误",
  "Internal server error",
  "Internal Server Error",
]);

export function getErrorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof Error)) {
    return fallback;
  }
  const message = error.message.trim();
  return message && !INTERNAL_ERROR_PLACEHOLDERS.has(message)
    ? message
    : fallback;
}

export type ResourceAccessFailure =
  | "authentication_required"
  | "forbidden";

export interface ResourceFailure {
  access: ResourceAccessFailure | null;
  message: string;
}

export type MutationFailureEffect =
  | "accepted"
  | "committed"
  | "not_applied"
  | "unknown";

export interface MutationFailure {
  category: string | null;
  code: string | null;
  effect: MutationFailureEffect;
  message: string;
  transportRequestId: string | null;
}

export function getResourceFailure(
  error: unknown,
  fallback: string,
): ResourceFailure {
  return {
    access: classifyResourceAccessFailure(error),
    message: getErrorMessage(error, fallback),
  };
}

/**
 * 把服务端 FailureCore 或本地传输失败投影为修改结果事实。
 *
 * 未识别的 effect 与普通异常一律保守降级为 unknown。这里不解释、更不会执行
 * resolution.action；具体恢复动作只能由持有领域状态的调用方显式决定。
 */
export function projectMutationFailure(
  error: unknown,
  fallback: string,
): MutationFailure {
  if (error instanceof ApiTransportError) {
    return {
      category: error.category,
      code: null,
      effect: "unknown",
      message: getErrorMessage(error, fallback),
      transportRequestId: error.transportRequestId,
    };
  }

  const structured = error instanceof ApiRequestError || error instanceof UnauthorizedError
    ? error
    : null;
  return {
    category: structured?.failure?.category ?? null,
    code: structured?.failure?.code ?? null,
    effect: knownMutationEffect(structured?.failure?.effect),
    message: getErrorMessage(error, fallback),
    transportRequestId: structured?.transportRequestId ?? null,
  };
}

export function getMutationFailure(
  error: unknown,
  fallback: string,
): MutationFailure {
  return projectMutationFailure(error, fallback);
}

function knownMutationEffect(value: string | undefined): MutationFailureEffect {
  switch (value) {
    case "accepted":
    case "committed":
    case "not_applied":
    case "unknown":
      return value;
    default:
      return "unknown";
  }
}

function classifyResourceAccessFailure(
  error: unknown,
): ResourceAccessFailure | null {
  if (error instanceof UnauthorizedError) {
    return "authentication_required";
  }
  if (!(error instanceof ApiRequestError)) {
    return null;
  }
  if (error.status === 401 || error.failure?.category === "authentication") {
    return "authentication_required";
  }
  if (error.status === 403 || error.failure?.category === "authorization") {
    return "forbidden";
  }
  return null;
}
