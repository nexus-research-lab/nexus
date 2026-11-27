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
  return { canCreateConversation: true, canOpenSubagents: true, isMembersLoading: false, scopeKey: "a",
    onCreateConversation: vi.fn(async () => null), onManageMembers: vi.fn(), onOpenAuxiliaryTab: vi.fn() };
}
function view(props: ComponentProps<typeof RoomMobileActionsMenu>, locale: "en" | "zh" = "en") {
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}>
    <RoomMobileActionsMenu {...props} />
  </I18N_CONTEXT.Provider>;
}

it("omits new conversation for the single main window", async () => {
  const user = userEvent.setup();
  const props = { ...setup(), canCreateConversation: false };
  render(view(props));
  await user.click(screen.getByRole("button", { name: "More actions" }));
  expect(screen.queryByRole("menuitem", { name: MESSAGES.en["room.new_conversation"] })).toBeNull();
  expect(screen.getAllByRole("menuitem")[0]?.textContent).toBe(MESSAGES.en["room.members"]);
});
