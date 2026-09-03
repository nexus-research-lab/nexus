// INPUT: Agent 目录卡片的身份、元数据与详情/聊天/建群动作。
// OUTPUT: 证明网格和列表复用共享视觉原语且动作边界互不串联。
// POS: Contacts Agent 卡片 DOM 合同；不覆盖目录筛选或详情保存。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";

import { ContactsAgentCard } from "./contacts-agent-card";

const AGENT: Agent = {
  agent_id: "agent-1",
  avatar: null,
  business_tags: ["研究", "写作", "自动化"],
  created_at: 1_735_689_600_000,
  description: "整理资料并撰写报告",
  name: "Researcher",
  options: {
    allowed_tools: ["Read", "Write"],
    permission_mode: "acceptEdits",
    provider: "anthropic",
  },
  skills_count: 3,
  status: "idle",
  workspace_path: "/workspace/researcher",
};

function renderCard(view: "grid" | "list") {
  const actions = {
    onCreateTeam: vi.fn(),
    onOpenProfile: vi.fn(),
    onOpenRoom: vi.fn(),
  };
  const result = render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <ContactsAgentCard agent={AGENT} view={view} {...actions} />
    </I18N_CONTEXT.Provider>,
  );
  return { ...result, actions };
}

describe("ContactsAgentCard", () => {
  it("uses shared card actions, badges, and semantic radii in grid mode", async () => {
    const user = userEvent.setup();
    const { actions } = renderCard("grid");
    const profileActions = screen.getAllByRole("button", {
      name: "common.edit Researcher",
    });

    expect(profileActions[0].className).toContain("surface-radius-md");
    expect(profileActions[1].className).toContain("surface-radius-lg");
    expect(screen.getAllByText("研究")[0].className).toContain("rounded-full");
    await user.click(profileActions[0]);
    expect(actions.onOpenProfile).toHaveBeenCalledOnce();
  });

  it("keeps list-row navigation separate from communication actions", async () => {
    const user = userEvent.setup();
    const { actions, container } = renderCard("list");
    const row = container.querySelector("div[role='button']");

    expect(row).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "contacts.chat" }));
    expect(actions.onOpenRoom).toHaveBeenCalledOnce();
    expect(actions.onOpenProfile).not.toHaveBeenCalled();
    await user.click(row!);
    expect(actions.onOpenProfile).toHaveBeenCalledOnce();
  });
});
