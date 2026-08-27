// INPUT: 任意失败值与当前操作的用户可执行兜底文案。
// OUTPUT: 保留具体业务错误；收敛内部占位错误；只从 HTTP 事实投影访问失效。
// POS: Web 错误展示的最小边界；不根据浏览器网络提示猜测请求失败原因。
import { ApiRequestError, UnauthorizedError } from "@/lib/api/core/http-error";

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

export function getResourceFailure(
  error: unknown,
  fallback: string,
): ResourceFailure {
  return {
    access: classifyResourceAccessFailure(error),
    message: getErrorMessage(error, fallback),
  };
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
