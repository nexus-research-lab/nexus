import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
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
): Promise<UserPreferences> {
  return requestApi<UserPreferences>(SETTINGS_PREFERENCES_API_BASE_URL, {
    method: "PATCH",
    body: { ...params },
  });
}
