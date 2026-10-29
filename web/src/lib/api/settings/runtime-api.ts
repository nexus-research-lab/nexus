import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { NXSRuntimeStatus } from "@/types/settings/preferences";

const NXS_RUNTIME_STATUS_API_URL = `${getAgentApiBaseUrl()}/settings/runtime/nxs/status`;

export async function getNxsRuntimeStatusApi(includeSandbox = false): Promise<NXSRuntimeStatus> {
  return requestApi<NXSRuntimeStatus>(includeSandbox ? `${NXS_RUNTIME_STATUS_API_URL}?include_sandbox=true` : NXS_RUNTIME_STATUS_API_URL, {
    method: "GET",
    timeout_ms: 8_000,
  });
}
