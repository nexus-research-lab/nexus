import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type { NXSRuntimeStatus } from "@/types/settings/preferences";

const NXS_RUNTIME_API_BASE_URL = `${get_agent_api_base_url()}/settings/runtime/nxs`;

export async function get_nxs_runtime_status_api(): Promise<NXSRuntimeStatus> {
  return request_api<NXSRuntimeStatus>(`${NXS_RUNTIME_API_BASE_URL}/status`, {
    method: "GET",
    timeout_ms: 8_000,
  });
}

export async function download_nxs_runtime_api(): Promise<NXSRuntimeStatus> {
  return request_api<NXSRuntimeStatus>(`${NXS_RUNTIME_API_BASE_URL}/download`, {
    method: "POST",
    body: {},
    timeout_ms: 120_000,
  });
}
