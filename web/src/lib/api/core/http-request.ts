const DEFAULT_REQUEST_TIMEOUT_MS = 30_000;

type JsonRequestBody = Record<string, unknown> | unknown[];

export interface RequestApiOptions extends Omit<RequestInit, "body"> {
  body?: BodyInit | JsonRequestBody | null;
  notify_on_401?: boolean;
  timeout_ms?: number;
}

export interface PreparedHttpRequest {
  body: BodyInit | null | undefined;
  cleanup: () => void;
  didTimeout: () => boolean;
  headers: Headers;
  notifyOn401: boolean | undefined;
  requestInit: Omit<RequestApiOptions, "body" | "headers" | "notify_on_401" | "timeout_ms">;
  signal: AbortSignal | undefined;
}

export function prepareHttpRequest(
  init?: RequestApiOptions,
): PreparedHttpRequest {
  const {
    body: _body,
    headers: _headers,
    notify_on_401: notifyOn401,
    timeout_ms: timeoutMs,
    ...requestInit
  } = init ?? {};
  const { body, headers } = normalizeRequestPayload(init);
  const abort = buildAbortSignal(
    init?.signal,
    timeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
  );
  return {
    body,
    headers,
    notifyOn401,
    requestInit,
    ...abort,
  };
}

function normalizeRequestPayload(init?: RequestApiOptions): {
  body: BodyInit | null | undefined;
  headers: Headers;
} {
  const headers = new Headers(init?.headers);
  const sourceBody = init?.body;
  if (isJsonRequestBody(sourceBody)) {
    setJsonContentType(headers);
    return { body: JSON.stringify(sourceBody), headers };
  }
  if (typeof sourceBody === "string") {
    setJsonContentType(headers);
  }
  return { body: sourceBody, headers };
}

function setJsonContentType(headers: Headers): void {
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
}

function isJsonRequestBody(value: unknown): value is JsonRequestBody {
  if (!value || typeof value !== "object") {
    return false;
  }
  return Array.isArray(value) || !isNativeRequestBody(value);
}

function isNativeRequestBody(value: object): boolean {
  const matchesNativeBody =
    value instanceof FormData ||
    value instanceof URLSearchParams ||
    value instanceof Blob ||
    value instanceof ArrayBuffer ||
    ArrayBuffer.isView(value);
  if (matchesNativeBody) {
    return true;
  }
  return typeof ReadableStream !== "undefined" && value instanceof ReadableStream;
}

function buildAbortSignal(
  externalSignal: AbortSignal | null | undefined,
  timeoutMs: number,
): Pick<PreparedHttpRequest, "cleanup" | "didTimeout" | "signal"> {
  if (!externalSignal && timeoutMs <= 0) {
    return {
      signal: undefined,
      cleanup: () => {},
      didTimeout: () => false,
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
  if (externalSignal?.aborted) {
    controller.abort();
  } else if (externalSignal) {
    abortListener = () => controller.abort();
    externalSignal.addEventListener("abort", abortListener, { once: true });
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
    didTimeout: () => didTimeout,
  };
}
