import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";
import type { NXSRuntimeStatus } from "@/types/settings/preferences";

const NXS_RUNTIME_API_BASE_URL = `${getAgentApiBaseUrl()}/settings/runtime/nxs`;

export async function getNxsRuntimeStatusApi(): Promise<NXSRuntimeStatus> {
  return requestApi<NXSRuntimeStatus>(`${NXS_RUNTIME_API_BASE_URL}/status`, {
    method: "GET",
    timeout_ms: 8_000,
  });
}
