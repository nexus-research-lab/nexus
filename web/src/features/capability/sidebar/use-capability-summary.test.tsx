// INPUT: Deferred optional summary reads and mutation events.
// OUTPUT: Unknown counts never appear as zero; failed refresh preserves known counts.
// POS: Sidebar read projection regression, no network or navigation.
import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { getCapabilitySummaryApi, type CapabilitySummary } from "@/lib/api/capability/summary-api";
import { CAPABILITY_SUMMARY_MUTATED_EVENT } from "../capability-summary-events";
import { buildCapabilitySidebarItems } from "./capability-sidebar-model";
import { useCapabilitySummary } from "./use-capability-summary";
vi.mock("@/lib/api/capability/summary-api", () => ({ getCapabilitySummaryApi: vi.fn() }));
it("leaves unknown counts blank and retains a successful snapshot through refresh failure", async () => {
  const api = vi.mocked(getCapabilitySummaryApi);
  api.mockRejectedValueOnce(new Error("offline"));
  const { result } = renderHook(useCapabilitySummary);
  expect(result.current).toBeNull();
  await act(async () => { await Promise.resolve(); });
  expect(buildCapabilitySidebarItems(result.current, (key) => key).every(item => item.meta === "")).toBe(true);
  const snapshot: CapabilitySummary = { skills_count: 0, connected_connectors_count: 3, enabled_scheduled_tasks_count: 1,
    connected_channels_count: 0, configured_channels_count: 0, active_pairings_count: 0, workgraph_distillations_count: 0 };
  api.mockResolvedValueOnce(snapshot);
  act(() => { window.dispatchEvent(new Event(CAPABILITY_SUMMARY_MUTATED_EVENT)); });
  await waitFor(() => expect(result.current).toEqual(snapshot));
  expect(buildCapabilitySidebarItems(result.current, (key) => key)[0].meta).toBe("0");
  api.mockRejectedValueOnce(new Error("offline again"));
  await act(async () => { window.dispatchEvent(new Event(CAPABILITY_SUMMARY_MUTATED_EVENT)); });
  expect(api).toHaveBeenCalledTimes(3);
  expect(result.current).toEqual(snapshot);
});
