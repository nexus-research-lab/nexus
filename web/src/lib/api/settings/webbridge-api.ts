import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";

export interface BrowserExtensionStatus {
  connected: boolean;
  extension_version?: string;
  protocol_version: string;
}

const WEBBRIDGE_STATUS_API_URL = `${getAgentApiBaseUrl()}/internal/webbridge/status`;

export async function getBrowserExtensionStatusApi(): Promise<BrowserExtensionStatus> {
  return requestApi<BrowserExtensionStatus>(WEBBRIDGE_STATUS_API_URL, {
    method: "GET",
    timeout_ms: 5_000,
  });
}
