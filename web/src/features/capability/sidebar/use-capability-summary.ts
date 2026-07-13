"use client";

import { type Dispatch, type SetStateAction, useCallback, useEffect, useRef, useState } from "react";

import {
  getCapabilitySummaryApi,
  type CapabilitySummary,
} from "@/lib/api/capability/summary-api";

import { CAPABILITY_SUMMARY_MUTATED_EVENT } from "../capability-summary-events";
import {
  beginCapabilitySummaryRefresh,
  type CapabilitySummaryRefreshOptions,
  completeCapabilitySummaryRefresh,
  createCapabilitySummaryRefreshRuntime,
  EMPTY_CAPABILITY_SUMMARY,
} from "./capability-summary-refresh-model";

const CAPABILITY_SUMMARY_REVALIDATE_INTERVAL_MS = 60_000;

function applyCapabilitySummary(
  mounted: boolean,
  setSummary: Dispatch<SetStateAction<CapabilitySummary>>,
  summary: CapabilitySummary,
): void {
  if (mounted) {
    setSummary(summary);
  }
}

function resetCapabilitySummaryAfterError(
  mounted: boolean,
  options: CapabilitySummaryRefreshOptions,
  setSummary: Dispatch<SetStateAction<CapabilitySummary>>,
): void {
  if (mounted && options.resetOnError) {
    setSummary(EMPTY_CAPABILITY_SUMMARY);
  }
}

export function useCapabilitySummary(): CapabilitySummary {
  const runtimeRef = useRef(createCapabilitySummaryRefreshRuntime());
  const [summary, setSummary] = useState(EMPTY_CAPABILITY_SUMMARY);

  const refreshSummary = useCallback(async (
    initialOptions: CapabilitySummaryRefreshOptions = {},
  ): Promise<void> => {
    let options: CapabilitySummaryRefreshOptions | null = initialOptions;
    while (options) {
      const runtime = runtimeRef.current;
      const shouldStart = beginCapabilitySummaryRefresh(
        runtime,
        options,
        Date.now(),
        CAPABILITY_SUMMARY_REVALIDATE_INTERVAL_MS,
      );
      if (!shouldStart) {
        return;
      }
      try {
        const nextSummary = await getCapabilitySummaryApi();
        applyCapabilitySummary(runtime.mounted, setSummary, nextSummary);
      } catch {
        resetCapabilitySummaryAfterError(runtime.mounted, options, setSummary);
      }
      options = completeCapabilitySummaryRefresh(runtime);
    }
  }, []);

  useEffect(() => {
    const runtime = runtimeRef.current;
    runtime.mounted = true;
    void refreshSummary({ force: true, resetOnError: true });
    const handleSummaryMutation = () => {
      void refreshSummary({ force: true });
    };
    window.addEventListener(CAPABILITY_SUMMARY_MUTATED_EVENT, handleSummaryMutation);
    return () => {
      runtime.mounted = false;
      window.removeEventListener(CAPABILITY_SUMMARY_MUTATED_EVENT, handleSummaryMutation);
    };
  }, [refreshSummary]);

  useEffect(() => {
    const handleRevalidate = () => {
      if (document.visibilityState === "visible") {
        void refreshSummary();
      }
    };
    window.addEventListener("focus", handleRevalidate);
    document.addEventListener("visibilitychange", handleRevalidate);
    return () => {
      window.removeEventListener("focus", handleRevalidate);
      document.removeEventListener("visibilitychange", handleRevalidate);
    };
  }, [refreshSummary]);

  return summary;
}
