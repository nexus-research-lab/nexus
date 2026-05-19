import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";

const SYSTEM_VERSION_API_URL = `${get_agent_api_base_url()}/system/version`;

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

export async function get_system_version_api(): Promise<SystemVersionInfo> {
  return request_api<SystemVersionInfo>(SYSTEM_VERSION_API_URL, {
    method: "GET",
    notify_on_401: false,
  });
}
