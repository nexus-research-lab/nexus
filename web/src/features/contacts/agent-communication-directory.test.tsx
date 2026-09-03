// INPUT: 联系人/候选目录、加载状态，以及选择和添加命令。
// OUTPUT: 证明联络目录复用共享列表、状态和弹窗原语，并保持选择/提交行为。
// POS: Contacts 联络目录 DOM 合同；聊天、请求竞态与服务端写入由各自所有者测试。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { Agent, AgentContact } from "@/types/agent/agent";

import { AgentCommunicationDirectory } from "./agent-communication-directory";

const CURRENT_AGENT: Agent = {
  agent_id: "agent-main",
  created_at: 1,
  is_main: true,
  name: "nexus",
  options: {},
  status: "idle",
  workspace_path: "/workspace/nexus",
};

const CONTACT: AgentContact = {
  alias: "搭档",
  contact_agent_id: "agent-contact",
  created_at: "2026-01-01T00:00:00Z",
  display_name: "Researcher",
  id: "contact-1",
  name: "researcher",
  owner_agent_id: CURRENT_AGENT.agent_id,
  updated_at: "2026-01-01T00:00:00Z",
};

const CANDIDATE: Agent = {
  agent_id: "agent-writer",
  created_at: 2,
  display_name: "Writer",
  name: "writer",
  options: {},
  status: "idle",
  workspace_path: "/workspace/writer",
};

function renderDirectory(
  overrides: Partial<React.ComponentProps<typeof AgentCommunicationDirectory>> = {},
) {
  const props: React.ComponentProps<typeof AgentCommunicationDirectory> = {
    agent: CURRENT_AGENT,
    agents: [CURRENT_AGENT, CANDIDATE],
    contacts: [CONTACT],
    directoryFailure: null,
    isDirectoryLoading: false,
    onAddContact: vi.fn(async () => true),
    onRefresh: vi.fn(),
    onSelectContact: vi.fn(),
    pendingAgentId: null,
    selectedContactId: CONTACT.contact_agent_id,
    ...overrides,
  };
  const result = render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <AgentCommunicationDirectory {...props} />
    </I18N_CONTEXT.Provider>,
  );
  return { ...result, props };
}

describe("AgentCommunicationDirectory", () => {
  it("uses shared list rows and submits the selected candidate", async () => {
    const user = userEvent.setup();
    const { props } = renderDirectory();
    const contact = screen.getByRole("button", { name: "搭档" });

    expect(contact.className).toContain("radius-control-md");
    expect(contact.getAttribute("aria-pressed")).toBe("true");
    expect(contact.querySelector(".ui-type-section-title")).not.toBeNull();

    await user.click(screen.getByRole("button", {
      name: "agent_options.contact.add_friend",
    }));
    const dialog = screen.getByRole("dialog", {
      name: "agent_options.contact.add_friend",
    });
    const candidate = within(dialog).getByRole("button", { name: "Writer" });
    expect(candidate.getAttribute("aria-pressed")).toBe("false");

    await user.click(candidate);
    expect(candidate.getAttribute("aria-pressed")).toBe("true");
    await user.type(
      within(dialog).getByLabelText("agent_options.contact.alias"),
      "写作伙伴",
    );
    await user.click(within(dialog).getByRole("button", {
      name: "agent_options.contact.add_friend",
    }));

    expect(props.onAddContact).toHaveBeenCalledWith("agent-writer", "写作伙伴");
  });

  it("projects directory loading through the shared reduced-motion state", () => {
    renderDirectory({
      contacts: [],
      isDirectoryLoading: true,
      selectedContactId: null,
    });

    const loading = screen.getByRole("status");
    expect(loading.getAttribute("data-resource-state")).toBe("loading");
    expect(loading.querySelector("svg")?.getAttribute("class")).toContain(
      "motion-reduce:animate-none",
    );
  });
});
