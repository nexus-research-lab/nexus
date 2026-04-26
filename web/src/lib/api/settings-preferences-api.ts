import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type {
  UpdateUserPreferencesParams,
  UserPreferences,
} from "@/types/settings/preferences";

const SETTINGS_PREFERENCES_API_BASE_URL = `${get_agent_api_base_url()}/settings/preferences`;

export async function get_user_preferences_api(): Promise<UserPreferences> {
  return request_api<UserPreferences>(SETTINGS_PREFERENCES_API_BASE_URL, {
    method: "GET",
  });
}

export async function update_user_preferences_api(
  params: UpdateUserPreferencesParams,
): Promise<UserPreferences> {
  return request_api<UserPreferences>(SETTINGS_PREFERENCES_API_BASE_URL, {
    method: "PATCH",
    body: { ...params },
  });
}
