/**
 * =====================================================
 * @File   : http.ts
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

import { ApiResponse } from "@/types/system/api";

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

interface RequestApiOptions extends Omit<RequestInit, "body"> {
  body?: BodyInit | JsonRequestBody | null;
  notify_on_401?: boolean;
  timeout_ms?: number;
}

export class UnauthorizedError extends Error {
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

function emit_auth_required() {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(AUTH_REQUIRED_EVENT));
}

async function parse_response_body<T>(
  response: Response,
): Promise<ApiResponse<T> | ApiErrorPayload | null> {
  const raw_text = await response.text();
  if (!raw_text) {
    return null;
  }

  try {
    return JSON.parse(raw_text) as ApiResponse<T> | ApiErrorPayload;
  } catch {
    return {
      message:
        raw_text.trim() ||
        `请求失败: ${response.status} ${response.statusText}`,
    };
  }
}

function normalize_error_detail(value: unknown): string | null {
  if (typeof value === "string") {
    const normalized_value = value.trim();
    return normalized_value || null;
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

function to_record(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  return value as Record<string, unknown>;
}

function read_nested_error_detail(
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string | null {
  if (!payload || !("data" in payload)) {
    return null;
  }
  const nested_payload = to_record(payload.data);
  if (!nested_payload) {
    return null;
  }
  return normalize_error_detail(nested_payload.detail);
}

function read_error_request_id(
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string | null {
  if (!payload || !("data" in payload)) {
    return null;
  }
  const nested_payload = to_record(payload.data);
  if (!nested_payload) {
    return null;
  }
  return normalize_error_detail(nested_payload.request_id);
}

function append_request_id(message: string, request_id: string | null): string {
  if (!request_id) {
    return message;
  }
  return `${message}（request_id: ${request_id}）`;
}

function is_json_request_body(value: unknown): value is JsonRequestBody {
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

function should_set_json_content_type(
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

function normalize_request_payload(init?: RequestApiOptions): {
  body: BodyInit | null | undefined;
  headers: Headers;
} {
  const headers = new Headers(init?.headers);
  let body = init?.body;

  if (is_json_request_body(body)) {
    if (!headers.has("Content-Type")) {
      headers.set("Content-Type", "application/json");
    }
    body = JSON.stringify(body);
    return { body, headers };
  }

  if (!headers.has("Content-Type") && should_set_json_content_type(body)) {
    headers.set("Content-Type", "application/json");
  }

  return { body, headers };
}

function build_abort_signal(
  external_signal: AbortSignal | null | undefined,
  timeout_ms: number,
): {
  signal: AbortSignal | undefined;
  cleanup: () => void;
  did_timeout: () => boolean;
} {
  if (!external_signal && timeout_ms <= 0) {
    return {
      signal: undefined,
      cleanup: () => {},
      did_timeout: () => false,
    };
  }

  const controller = new AbortController();
  let timeout_id: ReturnType<typeof setTimeout> | null = null;
  let did_timeout = false;
  let abort_listener: (() => void) | null = null;

  if (timeout_ms > 0) {
    timeout_id = setTimeout(() => {
      did_timeout = true;
      controller.abort();
    }, timeout_ms);
  }

  if (external_signal) {
    if (external_signal.aborted) {
      controller.abort();
    } else {
      abort_listener = () => {
        controller.abort();
      };
      external_signal.addEventListener("abort", abort_listener, { once: true });
    }
  }

  return {
    signal: controller.signal,
    cleanup: () => {
      if (timeout_id) {
        clearTimeout(timeout_id);
      }
      if (external_signal && abort_listener) {
        external_signal.removeEventListener("abort", abort_listener);
      }
    },
    did_timeout: () => did_timeout,
  };
}

function build_error_message(
  response: Response,
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string {
  if (!payload) {
    return `请求失败: ${response.status} ${response.statusText}`;
  }

  const request_id = read_error_request_id(payload);

  const direct_detail =
    "detail" in payload ? normalize_error_detail(payload.detail) : null;
  if (direct_detail) {
    return append_request_id(direct_detail, request_id);
  }

  const nested_detail = read_nested_error_detail(payload);
  if (nested_detail) {
    return append_request_id(nested_detail, request_id);
  }

  const direct_message =
    "message" in payload ? normalize_error_detail(payload.message) : null;
  if (direct_message) {
    return append_request_id(direct_message, request_id);
  }
  return append_request_id(
    `请求失败: ${response.status} ${response.statusText}`,
    request_id,
  );
}

export async function request_api<T>(
  input: string,
  init?: RequestApiOptions,
): Promise<T> {
  const {
    notify_on_401,
    timeout_ms,
    body: _unused_body,
    headers: _unused_headers,
    ...request_init
  } = init ?? {};
  const { body, headers } = normalize_request_payload(init);
  const { signal, cleanup, did_timeout } = build_abort_signal(
    init?.signal,
    timeout_ms ?? DEFAULT_REQUEST_TIMEOUT_MS,
  );

  let response: Response;
  try {
    response = await fetch(input, {
      credentials: "include",
      ...request_init,
      body,
      headers,
      signal,
    });
  } catch (error) {
    cleanup();
    if (did_timeout()) {
      throw new Error("请求超时，请稍后重试");
    }
    throw error;
  }

  const payload = await parse_response_body<T>(response);
  cleanup();

  if (!response.ok) {
    const message = build_error_message(response, payload);
    if (response.status === 401) {
      if (notify_on_401 !== false) {
        emit_auth_required();
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

export function notify_auth_required() {
  emit_auth_required();
}
