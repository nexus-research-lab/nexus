import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import type {
  RuntimeSettings,
  UpdateRuntimeSettingsParams,
} from "@/types/settings/runtime";

const SETTINGS_RUNTIME_API_BASE_URL = `${getAgentApiBaseUrl()}/settings/runtime`;

export async function getRuntimeSettingsApi(): Promise<RuntimeSettings> {
  return requestApi<RuntimeSettings>(SETTINGS_RUNTIME_API_BASE_URL, {
    method: "GET",
  });
}

export async function updateRuntimeSettingsApi(
  params: UpdateRuntimeSettingsParams,
): Promise<RuntimeSettings> {
  return requestApi<RuntimeSettings>(SETTINGS_RUNTIME_API_BASE_URL, {
    method: "PATCH",
    body: { ...params },
  });
}
