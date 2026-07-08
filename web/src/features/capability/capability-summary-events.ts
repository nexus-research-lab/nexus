"use client";

export const CAPABILITY_SUMMARY_MUTATED_EVENT = "nexus:capability-summary-mutated";

export function notifyCapabilitySummaryMutated(detail?: Record<string, unknown>) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(CAPABILITY_SUMMARY_MUTATED_EVENT, { detail }));
}
