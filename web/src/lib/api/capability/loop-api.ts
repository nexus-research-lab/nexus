import { getAgentApiBaseUrl } from "@/config/runtime-endpoints";
import { requestApi } from "@/lib/api/core/http";
import type { LoopCatalogItem } from "@/types/capability/loop";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

function localeQuery(locale: string): string {
  return locale ? `?locale=${encodeURIComponent(locale)}` : "";
}

export async function listLoopsApi(locale: string): Promise<LoopCatalogItem[]> {
  return requestApi<LoopCatalogItem[]>(
    `${AGENT_API_BASE_URL}/capability/loops${localeQuery(locale)}`,
    { method: "GET" },
  );
}
export async function getLoopApi(slug: string, locale: string): Promise<LoopCatalogItem> {
  return requestApi<LoopCatalogItem>(
    `${AGENT_API_BASE_URL}/capability/loops/${encodeURIComponent(slug)}${localeQuery(locale)}`,
    { method: "GET" },
  );
}
