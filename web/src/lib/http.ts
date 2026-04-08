/**
 * =====================================================
 * @File   : http.ts
 * @Date   : 2026-04-07 18:24
 * @Author : leemysw
 * 2026-04-07 18:24   Create
 * =====================================================
 */

import { ApiResponse } from "@/types/api";

export const AUTH_REQUIRED_EVENT = "nexus:auth-required";

interface ApiErrorPayload {
  detail?: string;
  message?: string;
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

function build_error_message(
  response: Response,
  payload: ApiResponse<unknown> | ApiErrorPayload | null,
): string {
  if (!payload) {
    return `请求失败: ${response.status} ${response.statusText}`;
  }

  if ("detail" in payload && payload.detail) {
    return payload.detail;
  }
  if ("message" in payload && payload.message) {
    return payload.message;
  }
  return `请求失败: ${response.status} ${response.statusText}`;
}

export async function request_api<T>(
  input: string,
  init?: RequestApiOptions,
): Promise<T> {
  const response = await fetch(input, {
    credentials: "include",
    ...init,
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

  return payload.data;
}

export function notify_auth_required() {
  emit_auth_required();
}
