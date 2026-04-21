/**
 * =====================================================
 * @File   : capability-api.ts
 * @Date   : 2026-04-18 19:42
 * @Author : leemysw
 * 2026-04-18 19:42   Create
 * =====================================================
 */

import { get_agent_api_base_url } from "@/config/options";
import { request_api } from "@/lib/api/http";

const AGENT_API_BASE_URL = get_agent_api_base_url();

export interface CapabilitySummary {
  skills_count: number;
  connected_connectors_count: number;
  enabled_scheduled_tasks_count: number;
}

export async function get_capability_summary_api(): Promise<CapabilitySummary> {
  return request_api<CapabilitySummary>(
    `${AGENT_API_BASE_URL}/capability/summary`,
    {
      method: "GET",
    },
  );
}
