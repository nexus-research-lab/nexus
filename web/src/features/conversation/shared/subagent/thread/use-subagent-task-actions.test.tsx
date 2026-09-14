// INPUT: Ambiguous stop failure followed by unknown, active and terminal task observations.
// OUTPUT: Only a known terminal observation clears pending stop reconciliation; no implicit replay.
// POS: Task action lifecycle regression with mocked transport and real state projection.

import { act, renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { stopSubagentTaskApi } from "@/lib/api/conversation/subagent-task-api";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { SubagentTask } from "@/types/conversation/subagent-task";
import { useSubagentTaskActions } from "./use-subagent-task-actions";

vi.mock("@/lib/api/conversation/subagent-task-api", () => ({ stopSubagentTaskApi: vi.fn(), sendSubagentTaskMessageApi: vi.fn() }));
it("retains an ambiguous stop result across unknown and running observations until a known terminal state", async () => {
  vi.mocked(stopSubagentTaskApi).mockRejectedValueOnce(new Error("network"));
  const task: SubagentTask = { task_id: "exact-task", status: "running", runtime_kind: "nxs", capabilities: { observe: true, transcript: true, stop: true, resume: true, send_message: true } };
  const refresh = vi.fn().mockResolvedValue(undefined);
  const { result, rerender } = renderHook(({ status }) => useSubagentTaskActions({ refresh, source: { kind: "session", session_key: "exact-session" }, task: { ...task, status } }), {
    initialProps: { status: "running" },
    wrapper: ({ children }) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>{children}</I18N_CONTEXT.Provider>,
  });
  await act(async () => { expect(await result.current.stop()).toBe(false); });
  expect(result.current.error).toEqual({ action: "stop", effect: "unknown" });
  rerender({ status: "future_state" });
  expect(result.current.error).toEqual({ action: "stop", effect: "unknown" });
  await act(async () => { expect(await result.current.stop()).toBe(false); });
  rerender({ status: "running" });
  expect(result.current.error).toEqual({ action: "stop", effect: "unknown" });
  expect(stopSubagentTaskApi).toHaveBeenCalledTimes(1);
  expect(refresh).not.toHaveBeenCalled();
  rerender({ status: "completed" });
  expect(result.current.error).toBeNull();
});
