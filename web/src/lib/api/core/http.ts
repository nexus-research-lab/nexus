/** 统一 HTTP 编排边界，只负责 fetch 生命周期和鉴权恢复决策。 */

import {
  applyDesktopRequestHeaders,
  recoverDesktopSessionTokenError,
} from "@/config/desktop-runtime";

import { notifyAuthRequired } from "./http-auth";
import { ApiRequestError, UnauthorizedError } from "./http-error";
import {
  prepareHttpRequest,
  type RequestApiOptions,
} from "./http-request";
import {
  buildApiErrorMessage,
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
    if (request.didTimeout()) {
      throw new Error("请求超时，请稍后重试");
    }
    throw error;
  }

  let payload: ParsedApiResponse<T>;
  try {
    payload = await parseApiResponseBody<T>(response);
  } finally {
    request.cleanup();
  }

  if (!response.ok) {
    const message = buildApiErrorMessage(response, payload);
    if (response.status === 401) {
      rejectUnauthorized({
        input,
        message,
        notifyOn401: request.notifyOn401,
      });
    }
    throw new ApiRequestError(message, response.status);
  }
  return getApiResponseData(payload);
}

function rejectUnauthorized({
  input,
  message,
  notifyOn401,
}: {
  input: string;
  message: string;
  notifyOn401: boolean | undefined;
}): never {
  if (recoverDesktopSessionTokenError(message, input)) {
    throw new UnauthorizedError(message);
  }
  if (notifyOn401 !== false) {
    notifyAuthRequired();
  }
  throw new UnauthorizedError(message);
}
