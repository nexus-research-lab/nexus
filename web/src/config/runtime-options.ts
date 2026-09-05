// INPUT: 当前 owner 的 Agent 默认身份和持久偏好快照。
// OUTPUT: 已清洗的运行时默认值、偏好变更事件与 owner 切换时的清空入口。
// POS: 配置内存投影；头像由真实 Agent 资源消费，不保存无人读取的第二份状态。

import type { AgentOptions } from "@/types/agent/agent";
import type { AgentConversationDefaultDeliveryPolicy } from "@/types/agent/agent-conversation";
import {
  DEFAULT_WEB_SEARCH_PROVIDER,
  normalizeAgentRuntimeKind,
  type AgentRuntimeKind,
  type UserPreferences,
} from "@/types/settings/preferences";
import {
  DEFAULT_AGENT_ALLOWED_TOOLS,
  DEFAULT_AGENT_PERMISSION_MODE,
} from "@/lib/agent-options";
import {
  mergeAgentOptions,
  normalizeModelSelectionPreference,
} from "@/lib/settings/preferences-normalization";

let DEFAULT_AGENT_ID = "";
export const USER_PREFERENCES_CHANGED_EVENT = "nexus:user-preferences-changed";
let DEFAULT_CHAT_DELIVERY_POLICY: AgentConversationDefaultDeliveryPolicy = "queue";
let DEFAULT_AGENT_RUNTIME_KIND: AgentRuntimeKind = "nxs";
let DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED = false;
let DEFAULT_EMOTION_ENABLED = false;
let DEFAULT_BROWSER_CDP_ENABLED = false;
let DEFAULT_RUNTIME_SETTINGS: UserPreferences["runtime_settings"] = {
  nxs: { auto_memory_enabled: true, auto_dream_enabled: true, tool_search: false },
};
let DEFAULT_WEB_SEARCH: UserPreferences["web_search"] = {
  enabled: true,
  provider: DEFAULT_WEB_SEARCH_PROVIDER,
};
let DEFAULT_IMAGE_MODEL_SELECTION: UserPreferences["default_image_model_selection"];
let DEFAULT_VISION_MODEL_SELECTION: UserPreferences["default_vision_model_selection"];
let DEFAULT_BACKGROUND_MODEL_SELECTION: UserPreferences["default_background_model_selection"];
let DEFAULT_PREFERENCES_VERSION: number | undefined;
let DEFAULT_AGENT_OPTIONS: Partial<AgentOptions> = {
  permission_mode: DEFAULT_AGENT_PERMISSION_MODE,
  allowed_tools: [...DEFAULT_AGENT_ALLOWED_TOOLS],
  disallowed_tools: [],
  setting_sources: ["project"],
};

export interface RuntimeOptionsSource {
  default_agent_id: string;
  preferences?: UserPreferences | null;
}

export function getDefaultAgentId(): string {
  return DEFAULT_AGENT_ID;
}

export function getInitialAgentOptions(): Partial<AgentOptions> {
  return mergeAgentOptions({}, DEFAULT_AGENT_OPTIONS);
}

export function getDefaultChatDeliveryPolicy(): AgentConversationDefaultDeliveryPolicy {
  return DEFAULT_CHAT_DELIVERY_POLICY;
}

export function getDefaultAgentRuntimeKind(): AgentRuntimeKind {
  return DEFAULT_AGENT_RUNTIME_KIND;
}

export function getUserPreferences(): UserPreferences {
  return {
    version: DEFAULT_PREFERENCES_VERSION,
    chat_default_delivery_policy: DEFAULT_CHAT_DELIVERY_POLICY,
    agent_runtime_kind: DEFAULT_AGENT_RUNTIME_KIND,
    agent_sdk_diagnostics_enabled: DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED,
    emotion_enabled: DEFAULT_EMOTION_ENABLED,
    browser_cdp_enabled: DEFAULT_BROWSER_CDP_ENABLED,
    runtime_settings: cloneRuntimeSettings(DEFAULT_RUNTIME_SETTINGS),
    web_search: DEFAULT_WEB_SEARCH ? { ...DEFAULT_WEB_SEARCH } : undefined,
    default_agent_options: getInitialAgentOptions(),
    default_image_model_selection: DEFAULT_IMAGE_MODEL_SELECTION,
    default_vision_model_selection: DEFAULT_VISION_MODEL_SELECTION,
    default_background_model_selection: DEFAULT_BACKGROUND_MODEL_SELECTION,
  };
}

export function setUserPreferences(preferences?: Partial<UserPreferences> | null): void {
  DEFAULT_PREFERENCES_VERSION = Number.isSafeInteger(preferences?.version)
    && (preferences?.version ?? 0) > 0
    ? preferences?.version
    : undefined;
  applyDeliveryPolicy(preferences);
  applyRuntimeKind(preferences);
  applyDiagnosticsPreference(preferences);
  applyEmotionPreference(preferences);
  applyBrowserCDPPreference(preferences);
  applyRuntimeSettings(preferences);
  applyWebSearch(preferences);
  applyModelSelections(preferences);
  DEFAULT_AGENT_OPTIONS = normalizeRuntimeAgentOptions(
    preferences?.default_agent_options,
  );
  notifyUserPreferencesChanged();
}

function applyDeliveryPolicy(
  preferences?: Partial<UserPreferences> | null,
): void {
  const policy = preferences?.chat_default_delivery_policy;
  if (policy !== undefined) {
    DEFAULT_CHAT_DELIVERY_POLICY = policy;
  }
}

function applyRuntimeKind(
  preferences?: Partial<UserPreferences> | null,
): void {
  if (preferences?.agent_runtime_kind !== undefined) {
    DEFAULT_AGENT_RUNTIME_KIND = normalizeAgentRuntimeKind(preferences.agent_runtime_kind);
  }
}

function applyDiagnosticsPreference(
  preferences?: Partial<UserPreferences> | null,
): void {
  if (preferences == null) {
    return;
  }
  DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED =
    preferences.agent_sdk_diagnostics_enabled === true;
}

function applyEmotionPreference(
  preferences?: Partial<UserPreferences> | null,
): void {
  if (preferences == null) {
    return;
  }
  DEFAULT_EMOTION_ENABLED = preferences.emotion_enabled === true;
}

function applyBrowserCDPPreference(
  preferences?: Partial<UserPreferences> | null,
): void {
  if (preferences == null) {
    return;
  }
  DEFAULT_BROWSER_CDP_ENABLED = preferences.browser_cdp_enabled === true;
}

function applyRuntimeSettings(
  preferences?: Partial<UserPreferences> | null,
): void {
  if (preferences?.runtime_settings === undefined) {
    return;
  }
  DEFAULT_RUNTIME_SETTINGS = normalizeRuntimeSettings(
    preferences.runtime_settings,
  );
}

function applyWebSearch(
  preferences?: Partial<UserPreferences> | null,
): void {
  if (preferences?.web_search === undefined) {
    return;
  }
  DEFAULT_WEB_SEARCH = preferences.web_search
    ? { ...preferences.web_search }
    : undefined;
}

function normalizeRuntimeSettings(
  settings?: UserPreferences["runtime_settings"],
): UserPreferences["runtime_settings"] {
  return {
    ...settings,
    nxs: {
      auto_memory_enabled: settings?.nxs?.auto_memory_enabled ?? true,
      auto_dream_enabled: settings?.nxs?.auto_dream_enabled ?? true,
      tool_search: settings?.nxs?.tool_search === true,
    },
  };
}

function cloneRuntimeSettings(
  settings?: UserPreferences["runtime_settings"],
): UserPreferences["runtime_settings"] {
  return settings
    ? Object.fromEntries(
        Object.entries(settings).map(([runtime, value]) => [
          runtime,
          value ? { ...value } : value,
        ]),
      )
    : undefined;
}

function applyModelSelections(
  preferences?: Partial<UserPreferences> | null,
): void {
  DEFAULT_IMAGE_MODEL_SELECTION = normalizeModelSelectionPreference(
    preferences?.default_image_model_selection,
  );
  DEFAULT_VISION_MODEL_SELECTION = normalizeModelSelectionPreference(
    preferences?.default_vision_model_selection,
  );
  DEFAULT_BACKGROUND_MODEL_SELECTION = normalizeModelSelectionPreference(
    preferences?.default_background_model_selection,
  );
}

export function isMainAgent(agentId?: string | null): boolean {
  return (agentId ?? "").trim() === DEFAULT_AGENT_ID;
}

export function resolveAgentId(agentId?: string | null): string {
  return (agentId ?? "").trim() || DEFAULT_AGENT_ID;
}

export function applyRuntimeOptions(
  source: RuntimeOptionsSource | null | undefined,
): void {
  const nextDefaultAgentId = source?.default_agent_id?.trim();
  if (!source || !nextDefaultAgentId) {
    throw new Error("运行时配置缺少 default_agent_id");
  }

  DEFAULT_AGENT_ID = nextDefaultAgentId;
  setUserPreferences(source.preferences);
}

/** Auth owner 变化时先移除上一账号的运行时默认值，再读取新 owner 配置。 */
export function resetRuntimeOptionsForOwnerChange(): void {
  DEFAULT_AGENT_ID = "";
  DEFAULT_CHAT_DELIVERY_POLICY = "queue";
  DEFAULT_AGENT_RUNTIME_KIND = "nxs";
  DEFAULT_AGENT_SDK_DIAGNOSTICS_ENABLED = false;
  DEFAULT_EMOTION_ENABLED = false;
  DEFAULT_BROWSER_CDP_ENABLED = false;
  DEFAULT_PREFERENCES_VERSION = undefined;
  DEFAULT_RUNTIME_SETTINGS = { nxs: { tool_search: false } };
  DEFAULT_WEB_SEARCH = {
    enabled: true,
    provider: DEFAULT_WEB_SEARCH_PROVIDER,
  };
  DEFAULT_IMAGE_MODEL_SELECTION = undefined;
  DEFAULT_VISION_MODEL_SELECTION = undefined;
  DEFAULT_BACKGROUND_MODEL_SELECTION = undefined;
  DEFAULT_AGENT_OPTIONS = {
    permission_mode: DEFAULT_AGENT_PERMISSION_MODE,
    allowed_tools: [...DEFAULT_AGENT_ALLOWED_TOOLS],
    disallowed_tools: [],
    setting_sources: ["project"],
  };
  notifyUserPreferencesChanged();
}

function normalizeRuntimeAgentOptions(
  options?: Partial<AgentOptions> | null,
): Partial<AgentOptions> {
  const source = options ?? {};
  return mergeAgentOptions({
    permission_mode: DEFAULT_AGENT_PERMISSION_MODE,
    allowed_tools: [...DEFAULT_AGENT_ALLOWED_TOOLS],
    disallowed_tools: [],
    setting_sources: ["project"],
  }, {
    ...source,
    permission_mode: source.permission_mode?.trim() || DEFAULT_AGENT_PERMISSION_MODE,
  });
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
