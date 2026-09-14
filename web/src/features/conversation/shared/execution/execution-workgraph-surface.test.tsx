// INPUT: Completed execution preview requests and changing session identity.
// OUTPUT: Same-frame clicks cannot duplicate extraction or resurrect a departed scope.
// POS: WorkGraph surface async boundary regression.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphWorkflowPreview } from "@/types/conversation/workgraph-workflow";
import { ExecutionWorkGraphSurface } from "./execution-workgraph-surface";
const preview = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api/conversation/execution-api", () => ({ previewWorkGraphWorkflowApi: preview }));
vi.mock("./use-workgraph-history-resource", () => ({ useWorkGraphHistoryResource: () => ({ history: [], error: null, isLoading: false, isStale: false, refresh: vi.fn() }) }));
vi.mock("./execution-workgraph-canvas", () => ({ ExecutionWorkGraphCanvas: () => <div>canvas</div> }));
vi.mock("./workgraph-distillation-dialog", () => ({ WorkGraphDistillationDialog: () => <div>draft dialog</div> }));
it("fences duplicate extraction and late completion after returning to the original session", async () => {
  let finish!: (value: WorkGraphWorkflowPreview) => void;
  preview.mockReturnValue(new Promise<WorkGraphWorkflowPreview>((resolve) => { finish = resolve; }));
  const execution = { id: "graph", status: "completed", objective: "Report", plan: { status: "active" }, work_items: [{ id: "work", status: "accepted", subject: "Report", position: 0 }] } as ExecutionView;
  const view = (sessionKey: string) => <I18nProvider><ExecutionWorkGraphSurface agents={[]} directory={{}} taskRuns={[]} resource={{ sessionKey, execution, error: null, isLoading: false, isStale: false, lastSuccessfulAt: null, refresh: vi.fn(), dismiss: vi.fn() }} /></I18nProvider>;
  const { rerender } = render(view("first"));
  const button = document.querySelector<HTMLButtonElement>("[data-workgraph-save-sketch]")!;
  act(() => { fireEvent.click(button); fireEvent.click(button); });
  expect(preview).toHaveBeenCalledOnce();
  rerender(view("second"));
  rerender(view("first"));
  await act(async () => finish({ preview_id: "old" } as WorkGraphWorkflowPreview));
  expect(screen.queryByText("draft dialog")).toBeNull();
});
