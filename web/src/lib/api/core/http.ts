/** 统一 HTTP 编排边界，只负责 fetch 生命周期和鉴权恢复决策。 */

import type { FailureCore } from "@/types/generated/protocol";

import {
  applyDesktopRequestHeaders,
  recoverDesktopSessionTokenError,
} from "@/config/desktop-runtime";

import { notifyAuthRequired } from "./http-auth";
import {
  ApiRequestError,
  ApiTransportError,
  UnauthorizedError,
} from "./http-error";
import {
  prepareHttpRequest,
  type PreparedHttpRequest,
  type RequestApiOptions,
} from "./http-request";
import {
  buildApiErrorMessage,
  getApiFailure,
  getApiResponseData,
  parseApiResponseBody,
  type ParsedApiResponse,
} from "./http-response";

export async function requestApi<T>(
  input: string,
  init?: RequestApiOptions,
): Promise<T> {
  const request = prepareHttpRequest(init);
  applyDesktopRequestHeaders(input, request.headers);

  let response: Response;
  try {
    response = await fetch(input, {
      credentials: "include",
      ...request.requestInit,
      body: request.body,
      headers: request.headers,
      signal: request.signal,
    });
  } catch (error) {
    request.cleanup();
    throw projectTransportError(error, request, "network");
  }

  let payload: ParsedApiResponse<T>;
  try {
    payload = await parseApiResponseBody<T>(response);
  } catch (error) {
    throw projectTransportError(error, request, "response_interrupted", response);
  } finally {
    request.cleanup();
  }

  if (!response.ok) {
    const message = buildApiErrorMessage(response, payload);
    const failure = getApiFailure(payload);
    if (response.status === 401) {
      rejectUnauthorized({
        failure,
        input,
        message,
        notifyOn401: request.notifyOn401,
      });
    }
    throw new ApiRequestError(message, response.status, failure);
  }
  return getApiResponseData(payload);
}

function rejectUnauthorized({
  failure,
  input,
  message,
  notifyOn401,
}: {
  failure: FailureCore | null;
  input: string;
  message: string;
  notifyOn401: boolean | undefined;
}): never {
  // 只有当前 v1 的稳定 code 可以驱动桌面恢复。未来版本仍保留作诊断，
  // 但不能仅因复用了同名字面值就触发刷新；旧文案兼容也只在没有
  // 结构化 FailureCore 时使用。
  const currentFailureCode = failure?.version === 1 ? failure.code : null;
  const legacyMessage = failure === null ? message : "";
  if (recoverDesktopSessionTokenError(legacyMessage, input, currentFailureCode)) {
    throw new UnauthorizedError(message, failure);
  }
  if (notifyOn401 !== false) {
    notifyAuthRequired();
  }
  throw new UnauthorizedError(message, failure);
}

function projectTransportError(
  error: unknown,
  request: PreparedHttpRequest,
  phase: "network" | "response_interrupted",
  response?: Response,
): unknown {
  // 调用方主动 Abort 继续抛出浏览器原始异常；scope 切换等既有取消逻辑
  // 不能被包装成需要展示或重试的产品失败。
  if (request.didExternalAbort()) {
    return error;
  }

  const transportRequestId = response?.headers.get("X-Request-ID") ?? null;
  const outcomeUnknown = request.transportFailureEffect === "unknown";
  if (request.didTimeout()) {
    return new ApiTransportError(
      outcomeUnknown ? "请求超时，暂时无法确认操作是否已经生效" : "请求超时，请重新加载",
      "timeout",
      request.transportFailureEffect,
      {
        status: response?.status ?? null,
        transportRequestId,
      },
    );
  }

  return new ApiTransportError(
    transportFailureMessage(phase, outcomeUnknown),
    phase,
    request.transportFailureEffect,
    {
      status: response?.status ?? null,
      transportRequestId,
    },
  );
}

function transportFailureMessage(
  phase: "network" | "response_interrupted",
  outcomeUnknown: boolean,
): string {
  if (outcomeUnknown) {
    return phase === "network"
      ? "连接中断，暂时无法确认操作是否已经生效"
      : "响应传输中断，暂时无法确认操作是否已经生效";
  }
  return phase === "network"
    ? "暂时无法连接服务，请重新加载"
    : "响应传输中断，请重新加载";
}
