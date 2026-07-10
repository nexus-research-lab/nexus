/**
 * 前端运行时地址解析。
 *
 * 常规部署优先走同源 `/agent` 代理。
 * 只有显式配置了绝对地址时，才会直连外部 API / WebSocket。
 */

import { requestApi } from "@/lib/api/http";
import { getDesktopRuntimeConfig } from "@/config/desktop-runtime";
import type { AgentOptions } from "@/types/agent/agent";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import { normalizeAgentRuntimeKind, type AgentRuntimeKind, type UserPreferences } from "@/types/settings/preferences";
import {
  DEFAULT_AGENT_ALLOWED_TOOLS,
  DEFAULT_AGENT_PERMISSION_MODE,
} from "@/features/agents/options/agent-options-constants";

let DEFAULT_AGENT_ID = "";
let DEFAULT_AGENT_AVATAR = "";
export const USER_PREFERENCES_CHANGED_EVENT = "nexus:user-preferences-changed";
let DEFAULT_CHAT_DELIVERY_POLICY: AgentConversationDefaultDeliveryPolicy = "queue";
let DEFAULT_AGENT_RUNTIME_KIND: AgentRuntimeKind = "nxs";
let DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED = false;
let DEFAULT_IMAGE_MODEL_SELECTION: UserPreferences["default_image_model_selection"];
let DEFAULT_BACKGROUND_MODEL_SELECTION: UserPreferences["default_background_model_selection"];
let DEFAULT_AGENT_OPTIONS: Partial<AgentOptions> = {
  permission_mode: DEFAULT_AGENT_PERMISSION_MODE,
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

function buildBrowserUrl(pathname: string, useWebsocketProtocol: boolean): string {
  if (typeof window === "undefined") {
    return pathname;
  }

  const normalizedPath = pathname.startsWith("/") ? pathname : `/${pathname}`;
  const origin = useWebsocketProtocol
    ? `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`
    : window.location.origin;
  return `${origin}${normalizedPath}`;
}

function resolveRuntimeUrl(rawUrl: string | undefined, fallbackPath: string, useWebsocketProtocol: boolean): string {
  const normalizedRawUrl = rawUrl?.trim();
  if (!normalizedRawUrl) {
    return buildBrowserUrl(fallbackPath, useWebsocketProtocol);
  }

  if (normalizedRawUrl.startsWith("/")) {
    return buildBrowserUrl(normalizedRawUrl, useWebsocketProtocol);
  }

  return normalizedRawUrl;
}

export function getAgentApiBaseUrl(): string {
  const desktopUrl = getDesktopRuntimeConfig()?.apiBaseUrl?.trim();
  if (desktopUrl) {
    return desktopUrl;
  }
  return resolveRuntimeUrl(import.meta.env.VITE_API_URL, DEFAULT_API_PATH, false);
}

export function getAgentWsUrl(): string {
  const desktopUrl = getDesktopRuntimeConfig()?.wsUrl?.trim();
  if (desktopUrl) {
    return desktopUrl;
  }
  return resolveRuntimeUrl(import.meta.env.VITE_WS_URL, DEFAULT_WS_PATH, true);
}

export function isStrictModeEnabled(): boolean {
  return ENABLE_STRICT_MODE;
}

export function getMessageHistoryRoundPageSize(): number {
  return MESSAGE_HISTORY_ROUND_PAGE_SIZE;
}

export function getMessageSendAckTimeoutMs(): number {
  return MESSAGE_SEND_ACK_TIMEOUT_MS;
}

export function getDefaultAgentId(): string {
  return DEFAULT_AGENT_ID;
}

export function getDefaultAgentAvatar(): string {
  return DEFAULT_AGENT_AVATAR;
}

function setDefaultAgentAvatar(avatar?: string | null): void {
  const normalizedAvatar = avatar?.trim();
  DEFAULT_AGENT_AVATAR = normalizedAvatar || "";
}

export function getInitialAgentOptions(): Partial<AgentOptions> {
  return cloneAgentOptions(DEFAULT_AGENT_OPTIONS);
}

export function getDefaultChatDeliveryPolicy(): AgentConversationDefaultDeliveryPolicy {
	return DEFAULT_CHAT_DELIVERY_POLICY;
}

export function getDefaultAgentRuntimeKind(): AgentRuntimeKind {
  return DEFAULT_AGENT_RUNTIME_KIND;
}

export function getUserPreferences(): UserPreferences {
  return {
    chat_default_delivery_policy: DEFAULT_CHAT_DELIVERY_POLICY,
    agent_runtime_kind: DEFAULT_AGENT_RUNTIME_KIND,
    agent_sdk_diagnostics_enabled: DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED,
    default_agent_options: getInitialAgentOptions(),
    default_image_model_selection: DEFAULT_IMAGE_MODEL_SELECTION,
    default_background_model_selection: DEFAULT_BACKGROUND_MODEL_SELECTION,
  };
}

export function setUserPreferences(preferences?: Partial<UserPreferences> | null): void {
  const policy = preferences?.chat_default_delivery_policy;
  if (policy !== undefined) {
    DEFAULT_CHAT_DELIVERY_POLICY = policy;
  }
  if (preferences?.agent_runtime_kind !== undefined) {
    DEFAULT_AGENT_RUNTIME_KIND = normalizeAgentRuntimeKind(preferences.agent_runtime_kind);
  }
  if (preferences !== undefined && preferences !== null) {
    DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED = preferences.agent_sdk_diagnostics_enabled === true;
  }
  DEFAULT_IMAGE_MODEL_SELECTION = normalizeModelSelectionPreference(preferences?.default_image_model_selection);
  DEFAULT_BACKGROUND_MODEL_SELECTION = normalizeModelSelectionPreference(preferences?.default_background_model_selection);
  DEFAULT_AGENT_OPTIONS = normalizeAgentOptions(preferences?.default_agent_options);
  notifyUserPreferencesChanged();
}

export function isMainAgent(agentId?: string | null): boolean {
  return (agentId ?? "").trim() === DEFAULT_AGENT_ID;
}

export function resolveAgentId(agentId?: string | null): string {
  return (agentId ?? "").trim() || DEFAULT_AGENT_ID;
}

export async function hydrateRuntimeOptions(): Promise<void> {
  const payload = await requestApi<{
    default_agent_id: string;
    default_agent_avatar?: string | null;
    preferences?: UserPreferences | null;
  }>(
    `${getAgentApiBaseUrl()}/runtime/options`,
    {
      method: "GET",
      notify_on_401: false,
    },
  );
  const nextDefaultAgentId = payload?.default_agent_id;
  if (!nextDefaultAgentId) {
    throw new Error("运行时配置缺少 default_agent_id");
  }

  DEFAULT_AGENT_ID = nextDefaultAgentId;
  setDefaultAgentAvatar(payload?.default_agent_avatar);
  setUserPreferences(payload?.preferences);
}

function cloneAgentOptions(options: Partial<AgentOptions>): Partial<AgentOptions> {
  return {
    ...options,
    allowed_tools: [...(options.allowed_tools ?? [])],
    disallowed_tools: [...(options.disallowed_tools ?? [])],
    setting_sources: [...(options.setting_sources ?? ["project"])],
  };
}

function normalizeAgentOptions(options?: Partial<AgentOptions> | null): Partial<AgentOptions> {
  const source = options ?? {};
  return {
    ...source,
    permission_mode: source.permission_mode?.trim() || DEFAULT_AGENT_PERMISSION_MODE,
    allowed_tools: [...(source.allowed_tools ?? DEFAULT_AGENT_ALLOWED_TOOLS)],
    disallowed_tools: [...(source.disallowed_tools ?? [])],
    setting_sources: [...(source.setting_sources ?? ["project"])],
  };
}

function normalizeModelSelectionPreference(
  selection?: UserPreferences["default_image_model_selection"] | null,
): UserPreferences["default_image_model_selection"] {
  const provider = selection?.provider?.trim();
  const model = selection?.model?.trim();
  if (!provider || !model) {
    return undefined;
  }
  return { provider, model };
}

function notifyUserPreferencesChanged(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent<UserPreferences>(
    USER_PREFERENCES_CHANGED_EVENT,
    { detail: getUserPreferences() },
  ));
}
