import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type {
  RuntimeSettings,
  UpdateRuntimeSettingsParams,
} from "@/types/settings/runtime";
import type { NXSRuntimeStatus } from "@/types/settings/preferences";

const SETTINGS_RUNTIME_API_BASE_URL = `${getAgentApiBaseUrl()}/settings/runtime`;
const NXS_RUNTIME_STATUS_API_URL = `${SETTINGS_RUNTIME_API_BASE_URL}/nxs/status`;

export async function getNxsRuntimeStatusApi(): Promise<NXSRuntimeStatus> {
  return requestApi<NXSRuntimeStatus>(NXS_RUNTIME_STATUS_API_URL, {
    method: "GET",
    timeout_ms: 8_000,
  });
}

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
    // users 根可能跨卷复制大量数据，不能沿用普通 API 的 30 秒超时。
    timeout_ms: 0,
  });
}
