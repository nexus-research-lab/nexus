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

interface ApiErrorPayload {
  detail?: unknown;
  message?: unknown;
  data?: {
    detail?: unknown;
    request_id?: unknown;
  };
}

interface RequestApiOptions extends RequestInit {
  notify_on_401?: boolean;
}

export class UnauthorizedError extends Error {
  constructor(message = "未登录或登录状态已过期") {
    super(message);
    this.name = "UnauthorizedError";
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
      message: raw_text.trim() || `请求失败: ${response.status} ${response.statusText}`,
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

function build_error_message(
  response: Response,
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string {
  if (!payload) {
    return `请求失败: ${response.status} ${response.statusText}`;
  }

  const request_id = read_error_request_id(payload);

  const direct_detail = "detail" in payload
    ? normalize_error_detail(payload.detail)
    : null;
  if (direct_detail) {
    return append_request_id(direct_detail, request_id);
  }

  const nested_detail = read_nested_error_detail(payload);
  if (nested_detail) {
    return append_request_id(nested_detail, request_id);
  }

  const direct_message = "message" in payload
    ? normalize_error_detail(payload.message)
    : null;
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
  // FormData 不需要手动设置 Content-Type，让浏览器自动设置 boundary
  const headers = init?.body instanceof FormData
    ? { ...init?.headers }
    : init?.headers;

  const response = await fetch(input, {
    credentials: "include",
    ...init,
    headers,
  });
  const payload = await parse_response_body<T>(response);

  if (!response.ok) {
    const message = build_error_message(response, payload);
    if (response.status === 401) {
      if (init?.notify_on_401 !== false) {
        emit_auth_required();
      }
      throw new UnauthorizedError(message);
    }
    throw new Error(message);
  }

  if (!payload || !("data" in payload)) {
    throw new Error("接口响应格式错误");
  }

  return payload.data as T;
}

export function notify_auth_required() {
  emit_auth_required();
}
