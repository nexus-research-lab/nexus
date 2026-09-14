// INPUT: WorkGraph source changes while an older request is unresolved.
// OUTPUT: The comparison never displays stale source data beside a new draft.
// POS: Artifact comparison identity regression with pure canvas stubs.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { getExecutionApi } from "@/lib/api/conversation/execution-api";
import type { ExecutionView } from "@/types/conversation/execution";
import type { WorkGraphArtifactContent } from "@/types/conversation/message/content";
import { WorkGraphArtifactBlock } from "./workgraph-artifact-block";
vi.mock("@/lib/api/conversation/execution-api", () => ({ getExecutionApi: vi.fn() }));
vi.mock("@/features/conversation/shared/execution/execution-workgraph-canvas", () => ({ ExecutionWorkGraphCanvas: ({ execution }: { execution: { id: string } }) => <div>{execution.id}</div> }));
vi.mock("@/features/conversation/shared/execution/named-workgraph-sketch", () => ({ NamedWorkGraphSketch: () => null }));
vi.mock("@/features/conversation/shared/execution/workgraph-workflow-canvas-model", () => ({ projectWorkGraphWorkflowCanvasExecution: () => ({ id: "draft" }) }));
it("ignores old source responses after switching artifacts", async () => {
  let old!: (value: ExecutionView) => void;
  let current!: (value: ExecutionView) => void;
  vi.mocked(getExecutionApi).mockImplementationOnce(() => new Promise((resolve) => { old = resolve; })).mockImplementationOnce(() => new Promise((resolve) => { current = resolve; }));
  const artifact = (id: string): WorkGraphArtifactContent => ({ type: "workgraph_artifact", state: "draft", operation: "extract", preview: { preview_id: id, slash_name: "report", title: "Report", source_session_key: "session", source_execution_id: id, objective: "Report", nodes: [], expires_at: "" } });
  const view = (id: string) => <I18nProvider><WorkGraphArtifactBlock artifact={artifact(id)} /></I18nProvider>;
  const { rerender } = render(view("old"));
  fireEvent.click(screen.getByRole("button", { name: /Compare|来源对照/i }));
  rerender(view("new"));
  await act(async () => current({ id: "Current source" } as unknown as ExecutionView));
  await act(async () => old({ id: "Old source" } as unknown as ExecutionView));
  expect(screen.getByText("Current source")).toBeTruthy();
  expect(screen.queryByText("Old source")).toBeNull();
});
