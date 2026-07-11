/** 统一 HTTP 请求边界，负责传输错误、鉴权失效和桌面端恢复协议。 */

import { ApiResponse } from "@/types/system/api";
import {
  applyDesktopRequestHeaders,
  recoverDesktopSessionTokenError,
} from "@/config/desktop-runtime";

export const AUTH_REQUIRED_EVENT = "nexus:auth-required";
const DEFAULT_REQUEST_TIMEOUT_MS = 30_000;

type JsonRequestBody = Record<string, unknown> | unknown[];

interface ApiErrorPayload {
  detail?: unknown;
  message?: unknown;
  data?: {
    detail?: unknown;
    request_id?: unknown;
  };
}

export interface RequestApiOptions extends Omit<RequestInit, "body"> {
  body?: BodyInit | JsonRequestBody | null;
  notify_on_401?: boolean;
  timeout_ms?: number;
}

class UnauthorizedError extends Error {
  constructor(message = "未登录或登录状态已过期") {
    super(message);
    this.name = "UnauthorizedError";
  }
}

export class ApiRequestError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiRequestError";
    this.status = status;
  }
}

function emitAuthRequired() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AUTH_REQUIRED_EVENT));
}

async function parseResponseBody<T>(
  response: Response,
): Promise<ApiResponse<T> | ApiErrorPayload | null> {
  const rawText = await response.text();
  if (!rawText) {
    return null;
  }

  try {
    return JSON.parse(rawText) as ApiResponse<T> | ApiErrorPayload;
  } catch {
    return {
      message:
        rawText.trim() ||
        `请求失败: ${response.status} ${response.statusText}`,
    };
  }
}

function normalizeErrorDetail(value: unknown): string | null {
  if (typeof value === "string") {
    const normalizedValue = value.trim();
    return normalizedValue || null;
  }
  if (value === null || value === undefined) {
    return null;
  }
  if (value instanceof Error) {
    return value.message.trim() || value.name;
  }
  if (typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return String(value);
    }
  }
  return String(value);
}

function toRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  return value as Record<string, unknown>;
}

function readNestedErrorDetail(
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string | null {
  if (!payload || !("data" in payload)) {
    return null;
  }
  const nestedPayload = toRecord(payload.data);
  if (!nestedPayload) {
    return null;
  }
  return normalizeErrorDetail(nestedPayload.detail);
}

function readErrorRequestId(
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string | null {
  if (!payload || !("data" in payload)) {
    return null;
  }
  const nestedPayload = toRecord(payload.data);
  if (!nestedPayload) {
    return null;
  }
  return normalizeErrorDetail(nestedPayload.request_id);
}

function appendRequestId(message: string, requestId: string | null): string {
  if (!requestId) {
    return message;
  }
  return `${message}（request_id: ${requestId}）`;
}

function isJsonRequestBody(value: unknown): value is JsonRequestBody {
  if (!value || typeof value !== "object") {
    return false;
  }
  if (Array.isArray(value)) {
    return true;
  }
  if (
    value instanceof FormData ||
    value instanceof URLSearchParams ||
    value instanceof Blob ||
    value instanceof ArrayBuffer ||
    ArrayBuffer.isView(value)
  ) {
    return false;
  }
  if (
    typeof ReadableStream !== "undefined" &&
    value instanceof ReadableStream
  ) {
    return false;
  }
  return true;
}

function shouldSetJsonContentType(
  body: BodyInit | null | undefined,
): boolean {
  if (!body) {
    return false;
  }
  if (
    body instanceof FormData ||
    body instanceof URLSearchParams ||
    body instanceof Blob ||
    body instanceof ArrayBuffer ||
    ArrayBuffer.isView(body)
  ) {
    return false;
  }
  if (typeof ReadableStream !== "undefined" && body instanceof ReadableStream) {
    return false;
  }
  return typeof body === "string";
}

function normalizeRequestPayload(init?: RequestApiOptions): {
  body: BodyInit | null | undefined;
  headers: Headers;
} {
  const headers = new Headers(init?.headers);
  let body = init?.body;

  if (isJsonRequestBody(body)) {
    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    body = JSON.stringify(body);
    return { body, headers };
  }

  if (!headers.has("Content-Type") && shouldSetJsonContentType(body)) {
    headers.set("Content-Type", "application/json");
  }

  return { body, headers };
}

function buildAbortSignal(
  externalSignal: AbortSignal | null | undefined,
  timeoutMs: number,
): {
  signal: AbortSignal | undefined;
  cleanup: () => void;
  did_timeout: () => boolean;
} {
  if (!externalSignal && timeoutMs <= 0) {
    return {
      signal: undefined,
      cleanup: () => {},
      did_timeout: () => false,
    };
  }

  const controller = new AbortController();
  let timeoutId: ReturnType<typeof setTimeout> | null = null;
  let didTimeout = false;
  let abortListener: (() => void) | null = null;

  if (timeoutMs > 0) {
    timeoutId = setTimeout(() => {
      didTimeout = true;
      controller.abort();
    }, timeoutMs);
  }

  if (externalSignal) {
    if (externalSignal.aborted) {
      controller.abort();
    } else {
      abortListener = () => {
        controller.abort();
      };
      externalSignal.addEventListener("abort", abortListener, { once: true });
    }
  }

  return {
    signal: controller.signal,
    cleanup: () => {
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
      if (externalSignal && abortListener) {
        externalSignal.removeEventListener("abort", abortListener);
      }
    },
    did_timeout: () => didTimeout,
  };
}

function buildErrorMessage(
  response: Response,
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string {
  if (!payload) {
    return `请求失败: ${response.status} ${response.statusText}`;
  }

  const requestId = readErrorRequestId(payload);

  const directDetail =
    "detail" in payload ? normalizeErrorDetail(payload.detail) : null;
  if (directDetail) {
    return appendRequestId(directDetail, requestId);
  }

  const nestedDetail = readNestedErrorDetail(payload);
  if (nestedDetail) {
    return appendRequestId(nestedDetail, requestId);
  }

  const directMessage =
    "message" in payload ? normalizeErrorDetail(payload.message) : null;
  if (directMessage) {
    return appendRequestId(directMessage, requestId);
  }
  return appendRequestId(
    `请求失败: ${response.status} ${response.statusText}`,
    requestId,
  );
}

export async function requestApi<T>(
  input: string,
  init?: RequestApiOptions,
): Promise<T> {
  const {
    notify_on_401: notifyOn401,
    timeout_ms: timeoutMs,
    body: _unused_body,
    headers: _unused_headers,
    ...requestInit
  } = init ?? {};
  const { body, headers } = normalizeRequestPayload(init);
  applyDesktopRequestHeaders(input, headers);
  const { signal, cleanup, did_timeout: didTimeout } = buildAbortSignal(
    init?.signal,
    timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
  );

  let response: Response;
  try {
    response = await fetch(input, {
      credentials: "include",
      ...requestInit,
      body,
      headers,
      signal,
    });
  } catch (error) {
    cleanup();
    if (didTimeout()) {
      throw new Error("请求超时，请稍后重试");
    }
    throw error;
  }

  const payload = await parseResponseBody<T>(response);
  cleanup();

  if (!response.ok) {
    const message = buildErrorMessage(response, payload);
    if (response.status === 401) {
      if (recoverDesktopSessionTokenError(message, input)) {
        throw new UnauthorizedError(message);
      }
      if (notifyOn401 !== false) {
        emitAuthRequired();
      }
      throw new UnauthorizedError(message);
    }
    throw new ApiRequestError(message, response.status);
  }

  if (!payload || !("data" in payload)) {
    throw new Error("接口响应格式错误");
  }

  return payload.data as T;
}

export function notifyAuthRequired() {
  emitAuthRequired();
}
