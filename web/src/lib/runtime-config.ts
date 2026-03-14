"use client";

/**
 * 前端运行时地址解析。
 *
 * 优先使用环境变量；若环境变量仍指向 localhost，
 * 且页面实际通过局域网 IP 打开，则自动对齐到当前页面 host。
 */

const LOCAL_HOSTS = new Set(["localhost", "127.0.0.1", "0.0.0.0", "::1"]);

function isLocalHost(hostname: string): boolean {
  return LOCAL_HOSTS.has(hostname);
}

function normalizeHost(hostname: string): string {
  if (!hostname || hostname === "0.0.0.0") {
    return "localhost";
  }

  return hostname;
}

function getBrowserHost(): string {
  if (typeof window === "undefined") {
    return "localhost";
  }

  return normalizeHost(window.location.hostname);
}

function alignUrlHost(rawUrl: string): string {
  if (typeof window === "undefined") {
    return rawUrl;
  }

  const browserHost = getBrowserHost();
  if (browserHost === "localhost") {
    return rawUrl;
  }

  try {
    const parsed = new URL(rawUrl);
    if (!isLocalHost(parsed.hostname)) {
      return rawUrl;
    }

    parsed.hostname = browserHost;
    return parsed.toString();
  } catch {
    return rawUrl;
  }
}

export function getAgentApiBaseUrl(): string {
  if (process.env.NEXT_PUBLIC_API_URL) {
    return alignUrlHost(process.env.NEXT_PUBLIC_API_URL);
  }

  return `http://${getBrowserHost()}:8010/agent/v1`;
}

export function getAgentWsUrl(): string {
  if (process.env.NEXT_PUBLIC_WS_URL) {
    return alignUrlHost(process.env.NEXT_PUBLIC_WS_URL);
  }

  return `ws://${getBrowserHost()}:8010/agent/v1/chat/ws`;
}
