import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";

const SYSTEM_VERSION_API_URL = `${getAgentApiBaseUrl()}/system/version`;

export interface SystemVersionInfo {
  project: string;
  version: string;
  git_commit?: string;
  build_date?: string;
  goos: string;
  goarch: string;
  target: string;
  release_url: string;
}

export async function getSystemVersionApi(): Promise<SystemVersionInfo> {
  return requestApi<SystemVersionInfo>(SYSTEM_VERSION_API_URL, {
    method: "GET",
    notify_on_401: false,
  });
}
