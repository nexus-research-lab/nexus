/**
 * INPUT: owner Preferences GET 与带可选持久 version 的 PATCH。
 * OUTPUT: 服务端 Preferences；新写入用强 If-Match 防止陈旧覆盖。
 * POS: Preferences HTTP transport 边界；version 不进入 patch 正文或业务身份。
 */
import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  UpdateUserPreferencesParams,
  UserPreferences,
} from "@/types/settings/preferences";

const SETTINGS_PREFERENCES_API_BASE_URL = `${getAgentApiBaseUrl()}/settings/preferences`;

export async function getUserPreferencesApi(): Promise<UserPreferences> {
  return requestApi<UserPreferences>(SETTINGS_PREFERENCES_API_BASE_URL, {
    method: "GET",
  });
}

export async function updateUserPreferencesApi(
  params: UpdateUserPreferencesParams,
  options?: { expectedVersion?: number },
): Promise<UserPreferences> {
  const expectedVersion = options?.expectedVersion;
  return requestApi<UserPreferences>(SETTINGS_PREFERENCES_API_BASE_URL, {
    method: "PATCH",
    body: { ...params },
    headers: Number.isSafeInteger(expectedVersion) && (expectedVersion ?? 0) > 0
      ? { "If-Match": `"preferences-${expectedVersion}"` }
      : undefined,
  });
}
