// INPUT: 完整成员目录、按 Agent 筛选的任务进程与当前语言。
// OUTPUT: 证明候选筛选不改变重名序号、精确任务切换与缺名来源回退。
// POS: Room 任务面板的真实组件集成回归，不模拟任务面板或成员菜单。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";
import type { ConversationTodoProcess } from "@/features/conversation/shared/todos/todo-projection-model";

import { RoomWorkspaceTaskPanel } from "./room-workspace-task-panel";

const members: Agent[] = ["hidden", "alpha", "beta"].map((agent_id, created_at) => ({
  agent_id, created_at, name: "Nova", options: {}, status: "idle", workspace_path: `/workspace/${agent_id}`,
}));
const processes: ConversationTodoProcess[] = ["alpha", "beta"].map((agentId, latestTaskEventIndex) => ({
  agentId,
  latestTaskEventIndex,
  todos: [{ content: `Task for ${agentId}`, status: "in_progress" }],
}));
const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
  .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.en[key]);

function panel(props: Parameters<typeof RoomWorkspaceTaskPanel>[0]) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t }}><RoomWorkspaceTaskPanel {...props} /></I18N_CONTEXT.Provider>;
}

describe("RoomWorkspaceTaskPanel identities", () => {
  it("numbers against the full directory while preserving process candidates and exact task selection", async () => {
    const user = userEvent.setup();
    render(panel({ roomMembers: members, processes, scopeKey: "room:session" }));
    const summary = screen.getByRole("button", { name: t("tasks.expand_panel") });
    expect(summary.textContent).toContain("Task for beta");
    await user.click(summary);
    await user.click(screen.getByRole("button", { name: /3 · Nova/ }));
    expect(screen.getAllByRole("menuitem")).toHaveLength(2);
    expect(screen.queryByRole("menuitem", { name: "1 · Nova" })).toBeNull();
    await user.click(screen.getByRole("menuitem", { name: "2 · Nova" }));
    expect(summary.textContent).toContain("Task for alpha");
    expect(summary.textContent).not.toContain("Task for beta");
    expect(screen.getByRole("button", { name: /2 · Nova/ }).getAttribute("title")).toBe("2 · Nova");
  });

  it("uses a localized source name for an unnamed sole member", () => {
    render(panel({ roomMembers: [{ ...members[1], name: "  " }], processes: [processes[0]], scopeKey: "room:session" }));
    const summary = screen.getByRole("button", { name: t("tasks.expand_panel") });
    expect(summary.querySelector('[data-workspace-task-agent-id="alpha"]')?.getAttribute("title")).toBe("Agent");
    expect(summary.textContent).toContain("Agent");
    expect(summary.textContent).toContain("Task for alpha");
  });
});
