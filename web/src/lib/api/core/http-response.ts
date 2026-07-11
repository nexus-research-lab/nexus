import type { ApiResponse } from "@/types/system/api";

interface ApiErrorPayload {
  detail?: unknown;
  message?: unknown;
  data?: {
    detail?: unknown;
    request_id?: unknown;
  };
}

interface HttpResponseMeta {
  status: number;
  statusText: string;
}

interface HttpResponseBodySource extends HttpResponseMeta {
  text: () => Promise<string>;
}

export type ParsedApiResponse<T> = ApiResponse<T> | ApiErrorPayload | null;

export async function parseApiResponseBody<T>(
  response: HttpResponseBodySource,
): Promise<ParsedApiResponse<T>> {
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

export function getApiResponseData<T>(payload: ParsedApiResponse<T>): T {
  if (!payload || !("data" in payload)) {
    throw new Error("接口响应格式错误");
  }
  return payload.data as T;
}

export function buildApiErrorMessage(
  response: HttpResponseMeta,
  payload: ParsedApiResponse<unknown>,
): string {
  const fallback = `请求失败: ${response.status} ${response.statusText}`;
  if (!payload) {
    return fallback;
  }

  const requestId = readNestedErrorValue(payload, "request_id");
  const candidates = [
    "detail" in payload ? normalizeErrorDetail(payload.detail) : null,
    readNestedErrorValue(payload, "detail"),
    "message" in payload ? normalizeErrorDetail(payload.message) : null,
    fallback,
  ];
  return appendRequestId(
    candidates.find((message) => Boolean(message)) ?? fallback,
    requestId,
  );
}

function readNestedErrorValue(
  payload: ParsedApiResponse<unknown>,
  key: "detail" | "request_id",
): string | null {
  if (!payload || !("data" in payload)) {
    return null;
  }
  return normalizeErrorDetail(toRecord(payload.data)?.[key]);
}

function normalizeErrorDetail(value: unknown): string | null {
  if (typeof value === "string") {
    return value.trim() || null;
  }
  if (value === null || value === undefined) {
    return null;
  }
  return normalizeNonStringErrorDetail(value);
}

function normalizeNonStringErrorDetail(value: unknown): string {
  if (value instanceof Error) {
    return value.message.trim() || value.name;
  }
  if (typeof value !== "object") {
    return String(value);
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function toRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object"
    ? value as Record<string, unknown>
    : null;
}

function appendRequestId(message: string, requestId: string | null): string {
  return requestId ? `${message}（request_id: ${requestId}）` : message;
}
