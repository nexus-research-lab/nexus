// INPUT: 五名 Room 成员与打开成员管理命令。
// OUTPUT: 证明成员头像栈复用共享 Header Button 并保留溢出计数。
// POS: GroupMemberAvatarStack DOM 行为测试；成员编辑事务由 Header 上层负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { GroupMemberAvatarStack } from "@/features/conversation/room/group/header/group-member-avatar-stack";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import type { Agent } from "@/types/agent/agent";

const MEMBERS: Agent[] = Array.from({ length: 5 }, (_, index) => ({
  agent_id: `agent-${index}`,
  created_at: index,
  name: `Agent ${index}`,
  options: {},
  status: "idle",
  workspace_path: `/workspace/agent-${index}`,
}));

describe("GroupMemberAvatarStack", () => {
  it("uses the shared header action without changing avatar projection", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    render(
      <I18nProvider>
        <GroupMemberAvatarStack members={MEMBERS} onClick={onClick} />
      </I18nProvider>,
    );

    const trigger = screen.getByRole("button", { name: /Members|成员/ });
    expect(trigger.className).toContain("h-9");
    expect(trigger.className).toContain("border-transparent");
    expect(screen.getByText("+1")).toBeTruthy();
    await user.click(trigger);
    expect(onClick).toHaveBeenCalledOnce();
  });

  it.each(["en", "zh"] as const)("keeps %s counts readable and avatar details decorative", (locale) => {
    const members = Array.from({ length: 1004 }, (_, i) => ({ ...MEMBERS[0], agent_id: `internal-${i}`, name: " " }));
    render(<I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key, params) =>
      Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]),
    }}><GroupMemberAvatarStack members={members} onClick={vi.fn()} /></I18N_CONTEXT.Provider>);
    const trigger = screen.getByRole("button", { name: locale === "en" ? "Members (1004)" : "成员（1004 人）" });
    expect(trigger.getAttribute("title")).toBe(trigger.getAttribute("aria-label"));
    expect(screen.queryByRole("img")).toBeNull();
    const avatars = screen.getAllByRole("img", { hidden: true });
    expect(avatars).toHaveLength(4);
    expect(avatars.every((avatar) => avatar.getAttribute("aria-label") === MESSAGES[locale]["agent.name_fallback"])).toBe(true);
    expect(avatars.every((avatar) => !avatar.hasAttribute("title"))).toBe(true);
    const counter = screen.getByText("+1000");
    expect(counter.className).toContain("min-h-[22px]");
    expect(counter.className).toContain("text-xs");
    expect(counter.className).not.toMatch(/w-5.5|text-\[8px\]/);
    expect(trigger.textContent).not.toContain("internal-");
  });

  it("keeps the empty entry available and blocks pointer/keyboard activation while loading or disabled", async () => {
    const user = userEvent.setup();
    const onClick = vi.fn();
    const view = render(<I18nProvider><GroupMemberAvatarStack members={[]} onClick={onClick} /></I18nProvider>);
    const trigger = screen.getByRole("button");
    expect(trigger.getAttribute("aria-label")).toContain("0");
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(onClick).toHaveBeenCalledOnce();
    view.rerender(<I18nProvider><GroupMemberAvatarStack isLoading members={[]} onClick={onClick} /></I18nProvider>);
    expect(trigger.getAttribute("aria-busy")).toBe("true");
    await user.click(trigger);
    await user.keyboard("{Enter}");
    expect(onClick).toHaveBeenCalledOnce();
    view.rerender(<I18nProvider><GroupMemberAvatarStack disabled members={MEMBERS} onClick={onClick} /></I18nProvider>);
    expect(trigger.hasAttribute("aria-busy")).toBe(false);
    expect(trigger.hasAttribute("disabled")).toBe(true);
    await user.click(trigger);
    expect(onClick).toHaveBeenCalledOnce();
  });

});
