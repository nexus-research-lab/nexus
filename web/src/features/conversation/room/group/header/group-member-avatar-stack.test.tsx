// INPUT: 五名 Room 成员与打开成员管理命令。
// OUTPUT: 证明成员头像栈复用共享 Header Button 并保留溢出计数。
// POS: GroupMemberAvatarStack DOM 行为测试；成员编辑事务由 Header 上层负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { GroupMemberAvatarStack } from "@/features/conversation/room/group/header/group-member-avatar-stack";
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
});
