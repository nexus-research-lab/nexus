// INPUT: HTTP 状态、响应正文与可选 FailureCore。
// OUTPUT: 用户可读错误正文、结构化 failure 与成功数据；诊断 ID 不拼入用户文案。
// POS: HTTP 响应解析边界；transport/request identity 只保留在结构化错误对象。

import type { ApiResponse } from "@/types/system/api";
import type {
  FailureCore,
  FailureRecoveryActor,
  FailureResolution,
} from "@/types/generated/protocol";

interface ApiErrorPayload {
  detail?: unknown;
  message?: unknown;
  data?: {
    detail?: unknown;
    failure?: unknown;
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

  // FailureCore 与旧响应都只返回用户文案；诊断关联号由 ApiRequestError
  // 单独持有，不能混入普通用户看到的错误说明。
  const candidates = [
    "detail" in payload ? normalizeErrorDetail(payload.detail) : null,
    readNestedErrorValue(payload, "detail"),
    "message" in payload ? normalizeErrorDetail(payload.message) : null,
    fallback,
  ];
  return candidates.find((message) => Boolean(message)) ?? fallback;
}

export function getApiFailure(
  payload: ParsedApiResponse<unknown>,
): FailureCore | null {
  if (!payload || !("data" in payload)) {
    return null;
  }
  return parseFailureCore(toRecord(payload.data)?.failure);
}

// FailureCore 使用开放字符串；新服务端值不能让旧客户端丢失整个失败响应。
export function parseFailureCore(value: unknown): FailureCore | null {
  const record = toRecord(value);
  if (!record) {
    return null;
  }
  const version = readFiniteNumber(record.version);
  const code = readNonEmptyString(record.code);
  const category = readNonEmptyString(record.category);
  const effect = readNonEmptyString(record.effect);
  if (
    version === null || version < 1 || code === null ||
    category === null || effect === null
  ) {
    return null;
  }

  const failure: FailureCore = { version, code, category, effect };
  const transportRequestID = readNonEmptyString(record.transport_request_id);
  if (transportRequestID !== null) {
    failure.transport_request_id = transportRequestID;
  }
  const retryAfterMS = readFiniteNumber(record.retry_after_ms);
  if (retryAfterMS !== null && retryAfterMS > 0) {
    failure.retry_after_ms = retryAfterMS;
  }
  const resolution = parseFailureResolution(record.resolution);
  if (resolution) {
    failure.resolution = resolution;
  }
  return failure;
}

function parseFailureResolution(value: unknown): FailureResolution | null {
  const record = toRecord(value);
  if (!record) {
    return null;
  }
  const actor = readNonEmptyString(record.actor);
  const action = readNonEmptyString(record.action);
  if (actor === null || action === null) {
    return null;
  }
  return { actor: actor as FailureRecoveryActor, action };
}

function readNestedErrorValue(
  payload: ParsedApiResponse<unknown>,
  key: "detail",
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

function readFiniteNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function readNonEmptyString(value: unknown): string | null {
  return typeof value === "string" ? value.trim() || null : null;
}

function toRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object"
    ? value as Record<string, unknown>
    : null;
}
