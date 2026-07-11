import type { CapabilitySummary } from "@/lib/api/capability/summary-api";

export const EMPTY_CAPABILITY_SUMMARY: CapabilitySummary = {
  active_pairings_count: 0,
  configured_channels_count: 0,
  connected_channels_count: 0,
  connected_connectors_count: 0,
  enabled_scheduled_tasks_count: 0,
  loops_count: 0,
  skills_count: 0,
};

export interface CapabilitySummaryRefreshOptions {
  force?: boolean;
  resetOnError?: boolean;
}

export interface CapabilitySummaryRefreshRuntime {
  inFlight: boolean;
  lastRefreshedAt: number;
  mounted: boolean;
  pendingForce: boolean;
}

const FORCE_REFRESH: CapabilitySummaryRefreshOptions = { force: true };

export function createCapabilitySummaryRefreshRuntime(): CapabilitySummaryRefreshRuntime {
  return {
    inFlight: false,
    lastRefreshedAt: 0,
    mounted: false,
    pendingForce: false,
  };
}

export function beginCapabilitySummaryRefresh(
  runtime: CapabilitySummaryRefreshRuntime,
  options: CapabilitySummaryRefreshOptions,
  now: number,
  revalidateIntervalMs: number,
): boolean {
  if (runtime.inFlight) {
    runtime.pendingForce ||= options.force === true;
    return false;
  }
  if (!options.force && now - runtime.lastRefreshedAt < revalidateIntervalMs) {
    return false;
  }
  runtime.inFlight = true;
  runtime.pendingForce = false;
  runtime.lastRefreshedAt = now;
  return true;
}

export function completeCapabilitySummaryRefresh(
  runtime: CapabilitySummaryRefreshRuntime,
): CapabilitySummaryRefreshOptions | null {
  runtime.inFlight = false;
  const shouldRunPendingRefresh = runtime.mounted && runtime.pendingForce;
  runtime.pendingForce = false;
  return shouldRunPendingRefresh ? FORCE_REFRESH : null;
}
