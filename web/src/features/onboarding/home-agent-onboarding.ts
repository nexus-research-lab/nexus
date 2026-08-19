import {
  clearHomeOnboardingAgentTaskDraft,
} from "./home-onboarding-agent-task";
import {
  clearHomeOnboardingRoomTaskDraft,
} from "./home-onboarding-room-task";

export const HOME_ONBOARDING_TOKEN_STORAGE_KEY =
  "nexus.home_onboarding_token";
export const HOME_ONBOARDING_COMPLETED_STORAGE_KEY =
  "nexus.home_onboarding_completed";
export const HOME_ONBOARDING_STAGE_STORAGE_KEY =
  "nexus.home_onboarding_stage";
export const HOME_ONBOARDING_RETURN_PATH_STORAGE_KEY =
  "nexus.home_onboarding_return_path";
export const HOME_ONBOARDING_ROLE_STORAGE_KEY =
  "nexus.home_onboarding_role";
export const HOME_ONBOARDING_PROVIDER_PRESET_STORAGE_KEY =
  "nexus.home_onboarding_provider_preset";
export const HOME_ONBOARDING_DEFAULT_MODEL_CONFIRMED_STORAGE_KEY =
  "nexus.home_onboarding_default_model_confirmed";
export const HOME_ONBOARDING_ROUND_ID = "nexus-home-onboarding";

export type HomeOnboardingStage =
  | "provider"
  | "token"
  | "provider_configuration"
  | "default_model"
  | "role"
  | "skill_guided"
  | "agent_name"
  | "agent_description"
  | "agent_style"
  | "agent_confirm"
  | "agent_created"
  | "room_idea"
  | "room_plan"
  | "room_ready"
  | "room_running"
  | "room_completed";

export const HOME_ONBOARDING_WELCOME_MESSAGE =
  "欢迎来到 Nexus。在开始旅程之前，请先选择你正在使用的模型厂商。";

export const HOME_ONBOARDING_INVALID_TOKEN_MESSAGE =
  "这个 Token 暂时无法连接所选模型厂商。请确认 Token 与厂商一致后重新发送。";

export const HOME_ONBOARDING_VALID_TOKEN_MESSAGE =
  "模型厂商已接入，并已同步可用模型。接下来我会带你完成 Nexus 主页引导。";

export const HOME_ONBOARDING_CONNECTING_MESSAGE =
  "收到，我正在验证 Token、连接模型厂商并同步可用模型，请稍等一下。";

export function buildHomeOnboardingTokenPrompt(
  providerDisplayName: string,
  keyUrl?: string,
): string {
  const prompt =
    `已选择 ${providerDisplayName}。现在请提供该厂商的模型 Token。它会保存到你的 Nexus 模型配置中，用来完成后续的主页引导。`;
  const normalizedKeyUrl = keyUrl?.trim();
  return normalizedKeyUrl
    ? `${prompt}\n\n还没有 API Key？[前往 ${providerDisplayName} 获取 API Key](${normalizedKeyUrl})`
    : prompt;
}

export function buildHomeOnboardingProviderKey(presetKey: string): string {
  const normalized = presetKey.trim().toLowerCase().replace(
    /[^a-z0-9_-]+/g,
    "-",
  );
  return `nexus-onboarding-${normalized || "provider"}`;
}

export function isHomeOnboardingCompleted(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  if (new URLSearchParams(window.location.search).get("onboarding") === "1") {
    return false;
  }
  const storedStage = window.localStorage.getItem(
    HOME_ONBOARDING_STAGE_STORAGE_KEY,
  );
  if (storedStage && storedStage !== "token") {
    return false;
  }
  return (
    window.localStorage.getItem(HOME_ONBOARDING_COMPLETED_STORAGE_KEY) ===
    "true"
  );
}

export function clearHomeOnboardingState(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(HOME_ONBOARDING_TOKEN_STORAGE_KEY);
  window.localStorage.removeItem(HOME_ONBOARDING_COMPLETED_STORAGE_KEY);
  window.localStorage.removeItem(HOME_ONBOARDING_STAGE_STORAGE_KEY);
  window.localStorage.removeItem(HOME_ONBOARDING_RETURN_PATH_STORAGE_KEY);
  window.localStorage.removeItem(HOME_ONBOARDING_ROLE_STORAGE_KEY);
  window.localStorage.removeItem(HOME_ONBOARDING_PROVIDER_PRESET_STORAGE_KEY);
  window.localStorage.removeItem(
    HOME_ONBOARDING_DEFAULT_MODEL_CONFIRMED_STORAGE_KEY,
  );
  clearHomeOnboardingAgentTaskDraft();
  clearHomeOnboardingRoomTaskDraft();
  notifyHomeOnboardingStateChange();
}

export function completeHomeOnboarding(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    HOME_ONBOARDING_COMPLETED_STORAGE_KEY,
    "true",
  );
  window.localStorage.removeItem(HOME_ONBOARDING_STAGE_STORAGE_KEY);
  window.localStorage.removeItem(HOME_ONBOARDING_RETURN_PATH_STORAGE_KEY);
  notifyHomeOnboardingStateChange();
}

export function maskHomeOnboardingToken(token: string): string {
  if (token.length <= 4) {
    return "••••";
  }
  return `${"•".repeat(Math.min(token.length - 4, 12))}${token.slice(-4)}`;
}

export function buildHomeOnboardingRoundId(index: number): string {
  return `${HOME_ONBOARDING_ROUND_ID}:${index}`;
}

export function getHomeOnboardingStage(): HomeOnboardingStage {
  if (typeof window === "undefined") {
    return "provider";
  }
  const stored = window.localStorage.getItem(HOME_ONBOARDING_STAGE_STORAGE_KEY);
  if (
    stored === "provider"
    || stored === "provider_configuration"
    || stored === "default_model"
    || stored === "skill_guided"
    || stored === "agent_name"
    || stored === "agent_description"
    || stored === "agent_style"
    || stored === "agent_confirm"
    || stored === "agent_created"
    || stored === "room_idea"
    || stored === "room_plan"
    || stored === "room_ready"
    || stored === "room_running"
    || stored === "room_completed"
  ) {
    return stored;
  }
  if (stored === "role") {
    return getHomeOnboardingRole() || isHomeOnboardingDefaultModelConfirmed()
      ? "role"
      : "default_model";
  }
  if (stored === "role_selected" && getHomeOnboardingRole()) {
    return "agent_name";
  }
  if (stored === "token" && getHomeOnboardingProviderPresetKey()) {
    return "token";
  }
  return "provider";
}

export function isHomeOnboardingDefaultModelConfirmed(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  return window.localStorage.getItem(
    HOME_ONBOARDING_DEFAULT_MODEL_CONFIRMED_STORAGE_KEY,
  ) === "true";
}

export function setHomeOnboardingDefaultModelConfirmed(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    HOME_ONBOARDING_DEFAULT_MODEL_CONFIRMED_STORAGE_KEY,
    "true",
  );
  notifyHomeOnboardingStateChange();
}

export function clearHomeOnboardingDefaultModelConfirmed(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(
    HOME_ONBOARDING_DEFAULT_MODEL_CONFIRMED_STORAGE_KEY,
  );
  notifyHomeOnboardingStateChange();
}

export function setHomeOnboardingStage(stage: HomeOnboardingStage): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(HOME_ONBOARDING_STAGE_STORAGE_KEY, stage);
  notifyHomeOnboardingStateChange();
}

export function setHomeOnboardingRole(role: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(HOME_ONBOARDING_ROLE_STORAGE_KEY, role);
  notifyHomeOnboardingStateChange();
}

export function getHomeOnboardingRole(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage.getItem(HOME_ONBOARDING_ROLE_STORAGE_KEY);
}

export function getHomeOnboardingProviderPresetKey(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.localStorage.getItem(
    HOME_ONBOARDING_PROVIDER_PRESET_STORAGE_KEY,
  );
}

export function setHomeOnboardingProviderPresetKey(
  presetKey: string,
): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(
    HOME_ONBOARDING_PROVIDER_PRESET_STORAGE_KEY,
    presetKey,
  );
  notifyHomeOnboardingStateChange();
}

export function clearHomeOnboardingProviderPresetKey(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(HOME_ONBOARDING_PROVIDER_PRESET_STORAGE_KEY);
  notifyHomeOnboardingStateChange();
}

export function rememberHomeOnboardingReturnPath(path?: string): void {
  if (typeof window === "undefined") {
    return;
  }
  const currentPath = path ?? window.location.pathname + window.location.search
    + window.location.hash;
  if (!currentPath.startsWith("/rooms/")) {
    return;
  }
  const url = new URL(currentPath, window.location.origin);
  url.searchParams.set("onboarding", "1");
  window.localStorage.setItem(
    HOME_ONBOARDING_RETURN_PATH_STORAGE_KEY,
    `${url.pathname}?${url.searchParams.toString()}${url.hash}`,
  );
  notifyHomeOnboardingStateChange();
}

export function getHomeOnboardingReturnPath(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  const path = window.localStorage.getItem(
    HOME_ONBOARDING_RETURN_PATH_STORAGE_KEY,
  );
  return path?.startsWith("/rooms/") ? path : null;
}

export function beginHomeOnboardingProviderConfiguration(): void {
  rememberHomeOnboardingReturnPath();
  setHomeOnboardingStage("provider_configuration");
}

export function notifyHomeOnboardingStateChange(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new Event("nexus:home-onboarding-state-change"));
}
