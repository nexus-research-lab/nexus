/**
 * 前端运行时地址解析。
 *
 * 常规部署优先走同源 `/agent` 代理。
 * 只有显式配置了绝对地址时，才会直连外部 API / WebSocket。
 */

import { request_api } from "@/lib/api/http";
import type { AgentOptions, AgentProvider } from "@/types/agent/agent";

export let DEFAULT_AGENT_ID = "";
export let DEFAULT_AGENT_PROVIDER: AgentProvider = "";

const DEFAULT_API_PATH = "/agent/v1";
const DEFAULT_WS_PATH = "/agent/v1/chat/ws";

function build_browser_url(pathname: string, use_websocket_protocol: boolean): string {
  if (typeof window === "undefined") {
    return pathname;
  }

  const normalized_path = pathname.startsWith("/") ? pathname : `/${pathname}`;
  const origin = use_websocket_protocol
    ? `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`
    : window.location.origin;
  return `${origin}${normalized_path}`;
}

function resolve_runtime_url(rawUrl: string | undefined, fallbackPath: string, use_websocket_protocol: boolean): string {
  const normalized_raw_url = rawUrl?.trim();
  if (!normalized_raw_url) {
    return build_browser_url(fallbackPath, use_websocket_protocol);
  }

  if (normalized_raw_url.startsWith("/")) {
    return build_browser_url(normalized_raw_url, use_websocket_protocol);
  }

  return normalized_raw_url;
}

export function get_agent_api_base_url(): string {
  return resolve_runtime_url(import.meta.env.VITE_API_URL, DEFAULT_API_PATH, false);
}

export function get_agent_ws_url(): string {
  return resolve_runtime_url(import.meta.env.VITE_WS_URL, DEFAULT_WS_PATH, true);
}

export function get_default_agent_id(): string {
  return DEFAULT_AGENT_ID;
}

export function get_default_agent_provider(): AgentProvider {
  return DEFAULT_AGENT_PROVIDER;
}

export function set_default_agent_provider(provider?: string | null): void {
  const normalized_provider = provider?.trim();
  DEFAULT_AGENT_PROVIDER = normalized_provider || "";
}

export function get_initial_agent_options(): Partial<AgentOptions> {
  return {
    permission_mode: "default",
  };
}

export function is_main_agent(agent_id?: string | null): boolean {
  return (agent_id ?? "").trim() === DEFAULT_AGENT_ID;
}

export function resolve_agent_id(agent_id?: string | null): string {
  return (agent_id ?? "").trim() || DEFAULT_AGENT_ID;
}

export async function hydrate_runtime_options(): Promise<void> {
  const payload = await request_api<{ default_agent_id: string; default_agent_provider?: string | null }>(
    `${get_agent_api_base_url()}/runtime/options`,
    {
      method: "GET",
      notify_on_401: false,
    },
  );
  const next_default_agent_id = payload?.default_agent_id;
  if (!next_default_agent_id || typeof next_default_agent_id !== "string") {
    throw new Error("运行时配置缺少 default_agent_id");
  }

  DEFAULT_AGENT_ID = next_default_agent_id;
  set_default_agent_provider(payload?.default_agent_provider);
}
