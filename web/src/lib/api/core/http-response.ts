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
    const payload = JSON.parse(rawText) as unknown;
    if (toRecord(payload)) {
      return payload as ApiResponse<T> | ApiErrorPayload;
    }
    // JSON 字符串、数字和数组也可能来自代理或损坏的上游响应。
    // 它们不是 Nexus envelope，不能把其中内容或随后产生的 TypeError 暴露给用户。
    return { message: "服务暂时无法完成请求" };
  } catch {
    return {
      // 未知响应可能是代理 HTML、堆栈或第三方正文，不能进入用户界面。
      // HTTP 状态仍由 ApiRequestError.status 保留给程序判断和诊断。
      message: "服务暂时无法完成请求",
    };
  }
}

export function getApiResponseData<T>(payload: ParsedApiResponse<T>): T {
  const record = toRecord(payload);
  if (!record || !("data" in record)) {
    throw new Error("接口响应格式错误");
  }
  return record.data as T;
}

export function buildApiErrorMessage(
  _response: HttpResponseMeta,
  payload: ParsedApiResponse<unknown>,
): string {
  const fallback = "服务暂时无法完成请求";
  if (!payload) {
    return fallback;
  }
  const record = toRecord(payload);
  if (!record) {
    return fallback;
  }

  // FailureCore 与旧响应都只返回用户文案；诊断关联号由 ApiRequestError
  // 单独持有，不能混入普通用户看到的错误说明。
  const candidates = [
    normalizeErrorDetail(record.detail),
    readNestedErrorValue(record, "detail"),
    normalizeErrorDetail(record.message),
    fallback,
  ];
  return candidates.find((message) => Boolean(message)) ?? fallback;
}

export function getApiFailure(
  payload: ParsedApiResponse<unknown>,
): FailureCore | null {
  const record = toRecord(payload);
  if (!record) {
    return null;
  }
  return parseFailureCore(toRecord(record.data)?.failure);
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
    version === null || !Number.isSafeInteger(version) || version < 1 || code === null ||
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
  payload: Record<string, unknown>,
  key: "detail",
): string | null {
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

function normalizeNonStringErrorDetail(_value: unknown): null {
  // API envelope 的用户文案合同是 string。对象、数组和其他未知值可能
  // 包含内部字段或秘密；机器上下文只能通过经过验证的结构化字段消费。
  return null;
}

function readFiniteNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function readNonEmptyString(value: unknown): string | null {
  return typeof value === "string" ? value.trim() || null : null;
}

function toRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === "object" && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}
