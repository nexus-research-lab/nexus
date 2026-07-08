/**
 * =====================================================
 * @File   : capability-api.ts
 * @Date   : 2026-04-18 19:42
 * @Author : leemysw
 * 2026-04-18 19:42   Create
 * =====================================================
 */

import { getAgentApiBaseUrl } from "@/config/options";
import { requestApi } from "@/lib/api/http";

const AGENT_API_BASE_URL = getAgentApiBaseUrl();

export interface CapabilitySummary {
  skills_count: number;
  connected_connectors_count: number;
  enabled_scheduled_tasks_count: number;
  connected_channels_count?: number;
  configured_channels_count?: number;
  active_pairings_count?: number;
  loops_count?: number;
}

export async function getCapabilitySummaryApi(): Promise<CapabilitySummary> {
  return requestApi<CapabilitySummary>(
    `${AGENT_API_BASE_URL}/capability/summary`,
    {
      method: "GET",
    },
  );
}
