// INPUT: Unknown deletion results and fresh owner directory snapshots.
// OUTPUT: Reads do not retry deletion; unresolved graphs stay locked until explicit new intent.
// POS: Named WorkGraph mutation recovery regression.
import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { WorkGraphWorkflow } from "@/types/conversation/workgraph-workflow";
import { deleteWorkGraphWorkflowApi } from "@/lib/api/conversation/execution-api";
import { useWorkGraphDeletion } from "./use-workgraph-deletion";
vi.mock("@/lib/api/conversation/execution-api", () => ({ deleteWorkGraphWorkflowApi: vi.fn() }));
it("keeps uncertain deletion locked through refresh and other feedback changes", async () => {
  vi.mocked(deleteWorkGraphWorkflowApi).mockRejectedValue(new Error("lost response"));
  const item = { id: "one" } as WorkGraphWorkflow;
  const reportFeedback = vi.fn();
  const { result } = renderHook(() => useWorkGraphDeletion({ onDeleted: vi.fn(), onRefresh: vi.fn(), reportFeedback }), { wrapper: I18nProvider });
  await act(async () => { await result.current.remove(item); });
  expect(result.current.blocked).toBe(true);
  act(() => result.current.reconcile([item]));
  expect(result.current.blocked).toBe(true);
  await act(async () => { await result.current.remove(item); });
  expect(deleteWorkGraphWorkflowApi).toHaveBeenCalledOnce();
  expect(result.current.feedback?.onDismiss).toBeUndefined();
  act(() => result.current.feedback?.action?.onClick());
  expect(result.current.blocked).toBe(false);
});
it("accepts absence as deletion evidence without issuing another delete", async () => {
  vi.mocked(deleteWorkGraphWorkflowApi).mockClear().mockRejectedValue(new Error("lost response"));
  const { result } = renderHook(() => useWorkGraphDeletion({ onDeleted: vi.fn(), onRefresh: vi.fn(), reportFeedback: vi.fn() }), { wrapper: I18nProvider });
  await act(async () => { await result.current.remove({ id: "one" } as WorkGraphWorkflow); });
  act(() => result.current.reconcile([]));
  expect(result.current.blocked).toBe(false);
  expect(result.current.feedback).toBeNull();
  expect(deleteWorkGraphWorkflowApi).toHaveBeenCalledOnce();
});
