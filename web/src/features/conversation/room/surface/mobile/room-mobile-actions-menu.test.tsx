// INPUT: 窄窗动作能力、加载阶段、作用域变化与真实点击/键盘事件。
// OUTPUT: 具名共享菜单、明确命令、防重/禁用项与关闭焦点的离线回归。
// POS: Room 窄窗动作装配测试；不执行会话创建或成员写入。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { RoomMobileActionsMenu } from "./room-mobile-actions-menu";

function setup() {
  return { canOpenSubagents: true, isMembersLoading: false, scopeKey: "a",
    onCreateConversation: vi.fn(async () => null), onManageMembers: vi.fn(), onOpenAuxiliaryTab: vi.fn() };
}
function view(props: ComponentProps<typeof RoomMobileActionsMenu>, locale: "en" | "zh" = "en") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <RoomMobileActionsMenu {...props} />
  </I18N_CONTEXT.Provider>;
}

it.each(["en", "zh"] as const)("names the trigger/menu in %s and keeps all six explicit commands", async (locale) => {
  const user = userEvent.setup();
  const props = setup();
  render(view(props, locale));
  const trigger = screen.getByRole("button", { name: MESSAGES[locale]["common.more_actions"] });
  const keys = ["room.new_conversation", "room.members", "room.workgraph", "subagents.label", "room.workspace", "room.about"] as const;
  for (const key of keys) {
    await user.click(trigger);
    expect(screen.getByRole("menu", { name: MESSAGES[locale]["common.more_actions"] })).toBeTruthy();
    expect(screen.getAllByRole("menuitem").map((item) => item.textContent)).toEqual(keys.map((item) => MESSAGES[locale][item]));
    await user.click(screen.getByRole("menuitem", { name: MESSAGES[locale][key] }));
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  }
  expect(props.onCreateConversation).toHaveBeenCalledOnce();
  expect(props.onManageMembers).toHaveBeenCalledOnce();
  expect(props.onOpenAuxiliaryTab.mock.calls).toEqual([["workgraph"], ["subagents"], ["workspace"], ["about"]]);
});

it("keeps WorkGraph available without a task source, skips unavailable actions and retains keyboard exit", async () => {
  const user = userEvent.setup();
  const props = { ...setup(), canOpenSubagents: false, isMembersLoading: true };
  render(view(props));
  const trigger = screen.getByRole("button", { name: "More actions" });
  await user.click(trigger);
  for (const name of [MESSAGES.en["room.members"], MESSAGES.en["subagents.label"]]) {
    const item = screen.getByRole("menuitem", { name });
    expect(item.hasAttribute("disabled")).toBe(true);
    await user.click(item);
  }
  expect(props.onManageMembers).not.toHaveBeenCalled();
  expect(props.onOpenAuxiliaryTab).not.toHaveBeenCalled();
  await user.keyboard("{Home}{ArrowDown}{Enter}");
  expect(props.onOpenAuxiliaryTab).toHaveBeenCalledExactlyOnceWith("workgraph");
  await user.click(trigger);
  await user.keyboard("{Escape}");
  expect(document.activeElement).toBe(trigger);
  expect(screen.queryByRole("menu")).toBeNull();
});

it("consumes old scope state, preserves language updates and uses current capability callbacks", async () => {
  const user = userEvent.setup();
  const props = setup();
  const rendered = render(view(props));
  await user.click(screen.getByRole("button", { name: "More actions" }));
  rendered.rerender(view(props, "zh"));
  expect(screen.getByRole("menu", { name: MESSAGES.zh["common.more_actions"] })).toBeTruthy();
  rendered.rerender(view({ ...props, scopeKey: "b" }));
  expect(screen.queryByRole("menu")).toBeNull();
  rendered.rerender(view(props));
  expect(screen.queryByRole("menu")).toBeNull();
  const next = { ...setup(), onManageMembers: undefined };
  rendered.rerender(view(next));
  await user.click(screen.getByRole("button", { name: "More actions" }));
  expect(screen.queryByRole("menuitem", { name: MESSAGES.en["room.members"] })).toBeNull();
  await user.keyboard("{Enter}");
  expect(next.onCreateConversation).toHaveBeenCalledOnce();
  expect(props.onCreateConversation).not.toHaveBeenCalled();
});
