// INPUT: Agent 目录卡片的身份、元数据与详情/聊天/建群动作。
// OUTPUT: 证明网格和列表复用共享视觉原语且动作边界互不串联。
// POS: Contacts Agent 卡片 DOM 合同；不覆盖目录筛选或详情保存。

import { act, render, screen, within } from "@testing-library/react";
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
  it.each(["grid", "list"] as const)("keeps one identity and independent keyboard/pointer actions in %s view", async (view) => {
    const user = userEvent.setup();
    const { actions } = renderCard(view);
    const open = screen.getByRole("button", { name: "common.edit Researcher" });
    expect(screen.getAllByRole("heading", { name: "Researcher" })).toHaveLength(1);
    expect(screen.getAllByText("研究")).toHaveLength(1);
    expect(screen.getByText("contacts.metadata.provider").tagName).toBe("DT");
    expect(screen.getByText("Anthropic").tagName).toBe("DD");
    const chat = screen.getByRole("button", { name: "contacts.chat Researcher" });
    const team = screen.getByRole("button", { name: "contacts.create_team Researcher" });
    await user.click(chat);
    act(() => team.focus());
    await user.keyboard("{Enter}");
    expect(actions.onOpenRoom).toHaveBeenCalledOnce();
    expect(actions.onCreateTeam).toHaveBeenCalledOnce();
    expect(actions.onOpenProfile).not.toHaveBeenCalled();
    act(() => open.focus());
    await user.keyboard("{Enter}");
    expect(actions.onOpenProfile).toHaveBeenCalledOnce();
    if (view === "grid") {
      expect(screen.getAllByRole("article")).toHaveLength(1);
      expect(within(screen.getByRole("article")).getAllByRole("button")).toHaveLength(3);
    } else {
      await user.tab();
      expect(screen.getByRole("tooltip").textContent).toBe("contacts.chat");
      expect(chat.getAttribute("title")).toBeNull();
    }
  });
});
