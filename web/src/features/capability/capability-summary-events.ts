"use client";

export const CAPABILITY_SUMMARY_MUTATED_EVENT = "nexus:capability-summary-mutated";

export function notify_capability_summary_mutated(detail?: Record<string, unknown>) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(CAPABILITY_SUMMARY_MUTATED_EVENT, { detail }));
}
