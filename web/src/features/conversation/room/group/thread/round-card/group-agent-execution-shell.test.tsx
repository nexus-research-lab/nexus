// INPUT: 真实 Agent 执行外壳的运行、stopping、终态和精确动作回调。
// OUTPUT: 单层紧凑控制条、防重停止、可读 Thread 开关及无动作空条的回归。
// POS: 使用真实 MessageItem 的 DOM 装配测试；不执行 runtime 或权限副作用。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { GroupAgentExecutionShell } from "./group-agent-execution-shell";

type Props = ComponentProps<typeof GroupAgentExecutionShell>;
const defaults: Props = { agentAvatar: null, agentId: "private-id", agentName: "Nova", isThreadActive: false, messages: [],
  onClickThread: vi.fn(), onPermissionResponse: vi.fn(() => true), onStopAgentRound: vi.fn(), pendingPermissions: [], roundId: "root", status: "streaming", timestamp: 1 };
const detailMessage: Props["messages"][number] = { message_id: "m", session_key: "session", agent_id: "private-id", round_id: "root", role: "assistant", timestamp: 1,
  content: [{ type: "thinking", thinking: "Private process" }], is_complete: true };
function view(props: Partial<Props> = {}) {
  return <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(),
    t: (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.en[key]) }}>
    <GroupAgentExecutionShell {...defaults} {...props} />
  </I18N_CONTEXT.Provider>;
}

it("uses one xs action group and disables only the in-flight stop command", async () => {
  const stop = vi.fn(); const thread = vi.fn();
  const user = userEvent.setup();
  const rendered = render(view({ onStopAgentRound: stop, onClickThread: thread }));
  const shell = rendered.container.querySelector("[data-room-agent-execution-shell]");
  const group = screen.getByRole("group", { name: MESSAGES.en["room.agent_actions"] });
  expect(group.className).toContain("min-h-8");
  const buttons = within(group).getAllByRole("button");
  expect(buttons.map((button) => button.getAttribute("data-room-agent-action"))).toEqual(["stop", "thread"]);
  buttons.forEach((button) => { expect(button.className).toContain("min-h-7"); expect(button.className).toContain("ui-type-metadata"); });
  expect(buttons[0].querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
  await user.click(buttons[0]);
  expect(stop).toHaveBeenCalledOnce();
  rendered.rerender(view({ isStopping: true, onStopAgentRound: stop, onClickThread: thread }));
  const stopping = within(group).getByRole("button", { name: MESSAGES.en["room.agent_stopping"] });
  expect(stopping.getAttribute("aria-busy")).toBe("true");
  expect(stopping.hasAttribute("disabled")).toBe(true);
  fireEvent.click(stopping);
  expect(stop).toHaveBeenCalledOnce();
  await user.click(within(group).getByRole("button", { name: "View Thread for Nova" }));
  expect(thread).toHaveBeenCalledOnce();
  expect(rendered.container.querySelector("[data-room-agent-execution-shell]")).toBe(shell);
});

it.each(["cancelled", "error"] as const)("keeps %s feedback readable and removes the active stop command", (status) => {
  render(view({ status, messages: [detailMessage], isThreadActive: true }));
  const group = screen.getByRole("group", { name: MESSAGES.en["room.agent_actions"] });
  const label = within(group).getByText(MESSAGES.en[status === "error" ? "room.agent_status_failed" : "room.agent_status_stopped"]);
  expect(label.className).toContain("ui-type-metadata");
  expect(label.className).toContain("ui-type-tone-muted");
  expect(within(group).queryByRole("button", { name: MESSAGES.en["room.agent_stop_action"] })).toBeNull();
  expect(within(group).getByRole("button", { name: "Close Thread for Nova" }).getAttribute("aria-expanded")).toBe("true");
  expect(screen.queryByText("Private process")).toBeNull();
});

it("does not show a separator or empty action group when capabilities and details disappear", () => {
  const rendered = render(view({ onStopAgentRound: undefined }));
  const group = screen.getByRole("group", { name: MESSAGES.en["room.agent_actions"] });
  expect(within(group).getAllByRole("button")).toHaveLength(1);
  expect(group.querySelector("[aria-hidden='true']")).toBeNull();
  rendered.rerender(view({ status: "done", messages: [{ ...detailMessage, content: [{ type: "text", text: "Final reply" }] }] }));
  expect(screen.queryByRole("group", { name: MESSAGES.en["room.agent_actions"] })).toBeNull();
  expect(screen.getByText("Final reply")).toBeTruthy();
});
