import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";
import type { LoopCatalogItem } from "@/types/capability/loop";

const AGENT_API_BASE_URL = get_agent_api_base_url();

function locale_query(locale: string): string {
  return locale ? `?locale=${encodeURIComponent(locale)}` : "";
}

export async function list_loops_api(locale: string): Promise<LoopCatalogItem[]> {
  return request_api<LoopCatalogItem[]>(
    `${AGENT_API_BASE_URL}/capability/loops${locale_query(locale)}`,
    { method: "GET" },
  );
}

export async function get_loop_api(slug: string, locale: string): Promise<LoopCatalogItem> {
  return request_api<LoopCatalogItem>(
    `${AGENT_API_BASE_URL}/capability/loops/${encodeURIComponent(slug)}${locale_query(locale)}`,
    { method: "GET" },
  );
}

