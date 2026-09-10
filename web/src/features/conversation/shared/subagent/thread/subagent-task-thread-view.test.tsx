// INPUT: Real subagent Thread, workspace evidence, capability controls and transcript states.
// OUTPUT: Source-consistent preview/download, localized shared states and exact pending/recovery behavior.
// POS: Task detail DOM integration; only download transport is mocked.

import { act, fireEvent, render, screen, within } from "@testing-library/react";
import type { ComponentProps, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { downloadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { useAgentStore } from "@/store/agent";
import type { AssistantMessage } from "@/types/conversation/message/entity";
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
function message(agentId: string, artifactOwner?: string): AssistantMessage {
  return {
    message_id: "message", session_key: "task-session", round_id: "round", agent_id: agentId, timestamp: 1, role: "assistant", is_complete: true,
    content: [{ type: "workspace_file_artifact", path: "reports/result.md", workspace_agent_id: artifactOwner }],
  };
}
function withMessages(task: SubagentTask, content: AssistantMessage): Model {
  return model({ task, messages: [content], rounds: [{ roundId: "round", messages: [content] }] });
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
  it.each(["desktop", "mobile"] as const)("keeps file identity independent of runtime/avatar identity in %s", (layout) => {
    const open = vi.fn();
    useAgentStore.setState({ current_agent_id: "unrelated-viewer" });
    const view = (task: SubagentTask, content: AssistantMessage) => localized(<SubagentTaskThreadView layout={layout} model={withMessages(task, content)} onBack={vi.fn()} onOpenWorkspaceFile={open} />);
    const { rerender } = render(view(TASK, message("host-author", "artifact-author")));
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenCalledExactlyOnceWith("reports/result.md", "artifact-author");
    fireEvent.click(screen.getByRole("button", { name: "Download result.md" }));
    expect(downloadWorkspaceFileApi).toHaveBeenCalledExactlyOnceWith("artifact-author", "reports/result.md", "result.md");
    rerender(view(TASK, message("host-author")));
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenLastCalledWith("reports/result.md", "host-author");
    rerender(view(TASK, message(" ")));
    fireEvent.click(screen.getByRole("button", { name: /^result\.md/ }));
    expect(open).toHaveBeenCalledTimes(3);
    expect(open).toHaveBeenLastCalledWith("reports/result.md", "host-author");
    rerender(view({ ...TASK, host_agent_id: "" }, message("")));
    const unavailable = screen.getByRole("button", { name: /^result\.md/ });
    expect(unavailable.hasAttribute("disabled")).toBe(true);
    fireEvent.click(unavailable);
    expect(open).toHaveBeenCalledTimes(3);
    expect(screen.queryByRole("button", { name: "Download result.md" })).toBeNull();
    expect(screen.getByText("The source workspace is unavailable, so this file cannot be opened.")).toBeTruthy();
  });

  it("localizes missing task names without exposing task/runtime IDs and uses the shared header avatar", () => {
    const task = { ...TASK, name: " ", description: " ", agent_type: " " };
    const view = (locale: I18nContextValue["locale"]) => localized(<SubagentTaskThreadView layout="desktop" model={model({ task })} onBack={vi.fn()} />, locale);
    const { container, rerender } = render(view("zh"));
    const header = container.querySelector("header")!;
    expect(header.textContent).toContain(MESSAGES.zh["agent.subagent_name_fallback"]);
    expect(header.textContent).not.toMatch(/private-task|runtime-child|host-author/);
    const avatar = header.querySelector('.h-8.w-8');
    expect(avatar?.className).toContain("h-8 w-8");
    expect(avatar?.className).not.toContain("h-7");
    rerender(view("en"));
    expect(header.textContent).toContain(MESSAGES.en["agent.subagent_name_fallback"]);
  });

  it("uses shared busy loading and empty states while preserving literal output evidence", () => {
    const view = (value: Model) => localized(<SubagentTaskThreadView layout="desktop" model={value} onBack={vi.fn()} />);
    const { container, rerender } = render(view(model({ isLoading: true })));
    expect(screen.getByText("Loading execution transcript...").closest('[data-resource-state]')?.getAttribute("aria-busy")).toBe("true");
    rerender(view(model()));
    expect(screen.getByText("No transcript yet").closest('[data-resource-state]')?.getAttribute("data-resource-state")).toBe("empty");
    rerender(view(model({ task: { ...TASK, capabilities: { ...TASK.capabilities, transcript: false } } })));
    expect(screen.getByText("Transcript unavailable").closest('[data-resource-state]')?.getAttribute("data-resource-state")).toBe("empty");
    const output = "Long line\n<script>plain output</script>";
    rerender(view(model({ detail: { task: TASK, messages: [], output } })));
    expect(container.querySelector("pre")?.textContent).toBe(output);
    expect(container.querySelector("pre")?.className).toContain("ui-type-code");
    expect(container.querySelector("script")).toBeNull();
  });

  it("keeps task controls usable, wraps the footer and identifies only the pending action as busy", () => {
    const value = model();
    const view = (value: Model) => localized(<SubagentTaskThreadView layout="desktop" model={value} onBack={vi.fn()} />);
    const { container, rerender } = render(view(value));
    fireEvent.click(screen.getByRole("button", { name: "Stop" }));
    fireEvent.click(screen.getByRole("button", { name: "Send instruction" }));
    expect(value.onStopRequest).toHaveBeenCalledOnce();
    expect(value.onSendRequest).toHaveBeenCalledOnce();
    const footer = container.querySelector("footer")!;
    expect(footer.className).not.toMatch(/backdrop-blur|color-mix/);
    expect(within(footer).getByText("Task controls use this exact task identity.").className).toContain("ui-type-metadata");
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


it.each(["en", "zh"] as const)("keeps unknown task state neutral and capability-driven in %s", (locale) => {
  const task = { ...TASK, status: "private_future_status" };
  const value = model({ task });
  const view = (value: Model) => localized(<SubagentTaskThreadView layout="desktop" model={value} onBack={vi.fn()} />, locale);
  const { container, rerender } = render(view(value));
  expect(screen.queryByRole("button", { name: MESSAGES[locale]["subagents.stop"] })).toBeNull();
  expect(screen.queryByRole("button", { name: MESSAGES[locale]["subagents.resume"] })).toBeNull();
  expect(screen.getByText(MESSAGES[locale]["subagents.controls_unknown_hint"])).toBeTruthy();
  fireEvent.click(screen.getByRole("button", { name: MESSAGES[locale]["subagents.send_message"] }));
  expect(value.onSendRequest).toHaveBeenCalledOnce();
  expect(container.textContent).not.toContain("private_future_status");
  expect(container.querySelector('header .h-8.w-8')?.className).not.toContain("status-running");
  rerender(view({ ...value, task: { ...task, capabilities: { ...task.capabilities, send_message: false } } }));
  expect(screen.queryByRole("button", { name: MESSAGES[locale]["subagents.send_message"] })).toBeNull();
});
