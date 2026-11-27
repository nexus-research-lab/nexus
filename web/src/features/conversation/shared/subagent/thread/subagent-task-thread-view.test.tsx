// INPUT: Real subagent Thread, workspace evidence, capability controls and transcript states.
// OUTPUT: Source-consistent preview/download, localized shared states and exact pending/recovery behavior.
// POS: Task detail DOM integration; only download transport is mocked.

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import type { ComponentProps, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useAgentStore } from "@/store/agent";
import type { SubagentTask } from "@/types/conversation/subagent-task";
import { SubagentTaskThreadView } from "./subagent-task-thread-view";

vi.mock("@/lib/api/agent/agent-api", () => ({ downloadWorkspaceFileApi: vi.fn().mockResolvedValue(undefined) }));

const TASK: SubagentTask = {
  task_id: "private-task", agent_id: "runtime-child", host_agent_id: "host-author", tool_use_id: "launch-tool", runtime_kind: "nxs", name: "Research task", status: "running",
  capabilities: { observe: true, transcript: true, resume: true, send_message: true, stop: true },
};
type Model = ComponentProps<typeof SubagentTaskThreadView>["model"];
function model(overrides: Partial<Model> = {}): Model {
  return {
    task: TASK, actions: { error: null, feedback: null, pendingAction: null, send: vi.fn().mockResolvedValue(true), stop: vi.fn().mockResolvedValue(true) },
    detail: null, error: null, isLoading: false, messages: [], rounds: [], sessionKey: "exact-task-thread", onRetry: vi.fn(), onSendRequest: vi.fn(), onStopRequest: vi.fn(), ...overrides,
  };
}
function localized(children: ReactNode, locale: I18nContextValue["locale"] = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
}
afterEach(() => {
  act(() => useAgentStore.setState({ current_agent_id: null }));
  vi.clearAllMocks();
});

describe("Subagent task details", () => {

  it("keeps task controls usable, wraps the footer and identifies only the pending action as busy", () => {
    const value = model();
    const view = (value: Model) => localized(<SubagentTaskThreadView layout="desktop" model={value} onBack={vi.fn()} />);
    const { container, rerender } = render(view(value));
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    fireEvent.click(screen.getByRole("button", { name: "Send instruction" }));
    expect(value.onStopRequest).toHaveBeenCalledOnce();
    expect(value.onSendRequest).toHaveBeenCalledOnce();
    const footer = container.querySelector("footer")!;

    for (const pendingAction of ["stop", "send"] as const) {
      rerender(view({ ...value, actions: { ...value.actions, pendingAction } }));
      const stop = screen.getByRole("button", { name: "Stop" });
      const send = screen.getByRole("button", { name: "Send instruction" });
      expect(stop.hasAttribute("disabled")).toBe(true);
      expect(send.hasAttribute("disabled")).toBe(true);
      expect(stop.getAttribute("aria-busy")).toBe(String(pendingAction === "stop"));
      expect(send.getAttribute("aria-busy")).toBe(String(pendingAction === "send"));
      fireEvent.click(stop);
      fireEvent.click(send);
    }
    expect(value.onStopRequest).toHaveBeenCalledOnce();
    expect(value.onSendRequest).toHaveBeenCalledOnce();
    rerender(view({ ...value, task: { ...TASK, status: "completed" } }));
    expect(screen.queryByRole("button", { name: "Stop" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Continue task" }));
    expect(value.onSendRequest).toHaveBeenCalledTimes(2);
    rerender(view({ ...value, task: { ...TASK, status: "deleted" } }));
    expect(within(footer).queryByRole("button")).toBeNull();
  });

  it.each(["unknown", "accepted", "committed"] as const)("keeps ambiguous stop state %s locked with one refresh action", (effect) => {
    const value = model();
    value.actions.error = { action: "stop", effect };
    render(localized(<SubagentTaskThreadView layout="desktop" model={value} onBack={vi.fn()} />));
    expect(screen.getByRole("button", { name: "Stop" }).hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: MESSAGES.en["subagents.refresh_task"] }));
    expect(value.onRetry).toHaveBeenCalledOnce();
    expect(value.actions.stop).not.toHaveBeenCalled();
  });

  it("preserves readable output after a query error and provides only one safe refresh", () => {
    const value = model({ detail: { task: TASK, messages: [], output: "Retained result" }, error: { retryable: true } });
    const { rerender } = render(localized(<SubagentTaskThreadView layout="desktop" model={value} onBack={vi.fn()} />));
    expect(screen.getByText("Retained result")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: MESSAGES.en["subagents.retry"] }));
    expect(value.onRetry).toHaveBeenCalledOnce();
    rerender(localized(<SubagentTaskThreadView layout="desktop" model={{ ...value, error: { retryable: false } }} onBack={vi.fn()} />));
    expect(screen.queryByRole("button", { name: MESSAGES.en["subagents.retry"] })).toBeNull();
    expect(screen.getByText("Retained result")).toBeTruthy();
    rerender(localized(<SubagentTaskThreadView layout="desktop" model={{ ...value, detail: null }} onBack={vi.fn()} />));
    expect(screen.getByText("This task's transcript could not be loaded")).toBeTruthy();
    expect(screen.queryByText("No transcript yet")).toBeNull();
  });
});
