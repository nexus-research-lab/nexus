/**
 * 前端运行时地址解析。
 *
 * 常规部署优先走同源 `/agent` 代理。
 * 只有显式配置了绝对地址时，才会直连外部 API / WebSocket。
 */

import { request_api } from "@/lib/api/http";
import type { AgentOptions, AgentProvider } from "@/types/agent/agent";
import type { AgentConversationDeliveryPolicy } from "@/types/agent/agent-conversation";
import type { UserPreferences } from "@/types/settings/preferences";
import { DEFAULT_AGENT_ALLOWED_TOOLS } from "@/features/agents/options/agent-options-constants";

export let DEFAULT_AGENT_ID = "";
export let DEFAULT_AGENT_AVATAR = "";
export let DEFAULT_AGENT_PROVIDER: AgentProvider = "";
export const USER_PREFERENCES_CHANGED_EVENT = "nexus:user-preferences-changed";
let DEFAULT_CHAT_DELIVERY_POLICY: AgentConversationDeliveryPolicy = "queue";
let DEFAULT_AGENT_OPTIONS: Partial<AgentOptions> = {
  permission_mode: "bypassPermissions",
  allowed_tools: [...DEFAULT_AGENT_ALLOWED_TOOLS],
  disallowed_tools: [],
  setting_sources: ["project"],
};

const DEFAULT_API_PATH = "/nexus/v1";
const DEFAULT_WS_PATH = "/nexus/v1/chat/ws";
const ENABLE_STRICT_MODE = false;
const MESSAGE_HISTORY_ROUND_PAGE_SIZE = 3;
// 与后端 protocol.ChatAckTimeoutMS 保持一致。
const MESSAGE_SEND_ACK_TIMEOUT_MS = 10000;

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

export function is_strict_mode_enabled(): boolean {
  return ENABLE_STRICT_MODE;
}

export function get_message_history_round_page_size(): number {
  return MESSAGE_HISTORY_ROUND_PAGE_SIZE;
}

export function get_message_send_ack_timeout_ms(): number {
  return MESSAGE_SEND_ACK_TIMEOUT_MS;
}

export function get_default_agent_id(): string {
  return DEFAULT_AGENT_ID;
}

export function get_default_agent_avatar(): string {
  return DEFAULT_AGENT_AVATAR;
}

export function get_default_agent_provider(): AgentProvider {
  return DEFAULT_AGENT_PROVIDER;
}

export function set_default_agent_avatar(avatar?: string | null): void {
  const normalized_avatar = avatar?.trim();
  DEFAULT_AGENT_AVATAR = normalized_avatar || "";
}

export function set_default_agent_provider(provider?: string | null): void {
  const normalized_provider = provider?.trim();
  DEFAULT_AGENT_PROVIDER = normalized_provider || "";
}

export function get_initial_agent_options(): Partial<AgentOptions> {
  return clone_agent_options(DEFAULT_AGENT_OPTIONS);
}

export function get_default_chat_delivery_policy(): AgentConversationDeliveryPolicy {
  return DEFAULT_CHAT_DELIVERY_POLICY;
}

export function get_user_preferences(): UserPreferences {
  return {
    chat_default_delivery_policy: DEFAULT_CHAT_DELIVERY_POLICY,
    default_agent_options: get_initial_agent_options(),
  };
}

export function set_user_preferences(preferences?: Partial<UserPreferences> | null): void {
  const policy = preferences?.chat_default_delivery_policy;
  if (policy === "queue" || policy === "guide" || policy === "interrupt" || policy === "auto") {
    DEFAULT_CHAT_DELIVERY_POLICY = policy;
  }
  DEFAULT_AGENT_OPTIONS = normalize_agent_options(preferences?.default_agent_options);
  notify_user_preferences_changed();
}

export function is_main_agent(agent_id?: string | null): boolean {
  return (agent_id ?? "").trim() === DEFAULT_AGENT_ID;
}

export function resolve_agent_id(agent_id?: string | null): string {
  return (agent_id ?? "").trim() || DEFAULT_AGENT_ID;
}

export async function hydrate_runtime_options(): Promise<void> {
  const payload = await request_api<{
    default_agent_id: string;
    default_agent_avatar?: string | null;
    default_agent_provider?: string | null;
    preferences?: UserPreferences | null;
  }>(
    `${get_agent_api_base_url()}/runtime/options`,
    {
      method: "GET",
      notify_on_401: false,
    },
  );
  const next_default_agent_id = payload?.default_agent_id;
  if (!next_default_agent_id) {
    throw new Error("运行时配置缺少 default_agent_id");
  }

  DEFAULT_AGENT_ID = next_default_agent_id;
  set_default_agent_avatar(payload?.default_agent_avatar);
  set_default_agent_provider(payload?.default_agent_provider);
  set_user_preferences(payload?.preferences);
}

function clone_agent_options(options: Partial<AgentOptions>): Partial<AgentOptions> {
  return {
    ...options,
    allowed_tools: [...(options.allowed_tools ?? [])],
    disallowed_tools: [...(options.disallowed_tools ?? [])],
    setting_sources: [...(options.setting_sources ?? ["project"])],
  };
}

function normalize_agent_options(options?: Partial<AgentOptions> | null): Partial<AgentOptions> {
  const source = options ?? {};
  return {
    ...source,
    permission_mode: source.permission_mode?.trim() || "bypassPermissions",
    allowed_tools: [...(source.allowed_tools ?? DEFAULT_AGENT_ALLOWED_TOOLS)],
    disallowed_tools: [...(source.disallowed_tools ?? [])],
    setting_sources: [...(source.setting_sources ?? ["project"])],
  };
}

function notify_user_preferences_changed(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent<UserPreferences>(
    USER_PREFERENCES_CHANGED_EVENT,
    { detail: get_user_preferences() },
  ));
}
