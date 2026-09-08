// INPUT: 已选且暂停的 Room 成员，以及成员/参与状态两个独立回调。
// OUTPUT: 证明共享列表行与嵌套 Choice 不会互相冒泡触发。
// POS: Room 成员选择 DOM 合同；表单归一化与持久提交由 controller 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { RoomMemberSelector } from "./room-member-selector";

describe("RoomMemberSelector", () => {
  it("keeps membership and participation choices independent", async () => {
    const user = userEvent.setup();
    const onToggleAgent = vi.fn();
    const onToggleParticipation = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{
          locale: "zh",
          setLocale: vi.fn(),
          t: (key, values) => values?.name ? `${key} ${values.name}` : key,
        }}
      >
        <RoomMemberSelector
          agents={[{ agent_id: "researcher", name: "研究员" }]}
          canManageParticipation
          onQueryChange={vi.fn()}
          onToggleAgent={onToggleAgent}
          onToggleParticipation={onToggleParticipation}
          pausedAgentIds={new Set(["researcher"])}
          query=""
          selectedAgentIds={new Set(["researcher"])}
        />
      </I18N_CONTEXT.Provider>,
    );

    const memberRow = screen.getByRole("button", {
      name: "room.agent_select_remove 研究员",
    });
    const participationChoice = screen.getByRole("button", {
      name: "room.resume_member 研究员",
    });
    expect(memberRow.className).toContain("min-h-10");
    expect(participationChoice.getAttribute("aria-pressed")).toBe("true");

    await user.click(participationChoice);
    expect(onToggleParticipation).toHaveBeenCalledWith("researcher");
    expect(onToggleAgent).not.toHaveBeenCalled();

    await user.click(memberRow);
    expect(onToggleAgent).toHaveBeenCalledWith("researcher");
  });
});
