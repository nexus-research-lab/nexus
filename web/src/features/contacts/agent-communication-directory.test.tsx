// INPUT: 联系人/候选目录、加载状态，以及选择和添加命令。
// OUTPUT: 证明联络目录复用共享列表、状态和弹窗原语，并保持选择/提交行为。
// POS: Contacts 联络目录 DOM 合同；聊天、请求竞态与服务端写入由各自所有者测试。

import { act, fireEvent, render, screen, within } from "@testing-library/react";
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
  const view = (value: typeof props) => (
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key, params) => `${key}${params ? ` ${Object.values(params).join(" ")}` : ""}` }}
    >
      <AgentCommunicationDirectory {...value} />
    </I18N_CONTEXT.Provider>
  );
  const result = render(view(props));
  return { ...result, props, rerenderDirectory: (next: Partial<typeof props>) => result.rerender(view({ ...props, ...next })) };
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

  it("keeps a refreshed snapshot searchable and offers an exact clear action", async () => {
    renderDirectory({ isDirectoryLoading: true, selectedContactId: null });
    await userEvent.type(screen.getByRole("searchbox", { name: "agent_options.contact.search_contacts" }), "missing");
    expect(screen.queryByText("agent_options.contact.loading_address_book")).toBeNull();
    const empty = screen.getByText("agent_options.contact.no_search_results").closest('[data-resource-state]')!;
    await userEvent.click(within(empty as HTMLElement).getByRole("button", { name: "common.clear" }));
    expect(screen.getByRole("button", { name: "搭档" })).toBeTruthy();
  });

  it.each([true, false])("only retains contacts when a directory failure is stale=%s", async (stale) => {
    const { props } = renderDirectory({ directoryFailure: { kind: "directory", stale } });
    expect(Boolean(screen.queryByRole("button", { name: "搭档" }))).toBe(stale);
    await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.retry_directory" }));
    expect(props.onRefresh).toHaveBeenCalledOnce();
    expect(props.onAddContact).not.toHaveBeenCalled();
  });

  it("distinguishes unmatched candidates and keeps the chosen identity explicit", async () => {
    renderDirectory();
    await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.add_friend" }));
    const dialog = screen.getByRole("dialog");
    const search = within(dialog).getByRole("searchbox", { name: "agent_options.contact.search_agents" });
    expect(document.activeElement).toBe(search);
    await userEvent.click(within(dialog).getByRole("button", { name: "Writer" }));
    await userEvent.type(search, "missing");
    expect(within(dialog).getByText("agent_options.contact.no_matching_agents")).toBeTruthy();
    expect(within(dialog).queryByText("agent_options.contact.no_available_agents")).toBeNull();
    expect(within(dialog).getByText("agent_options.contact.selected_agent Writer")).toBeTruthy();
    expect((within(dialog).getByRole("button", { name: "agent_options.contact.add_friend" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("locks the dialog and snapshots one add before the external pending prop updates", async () => {
    let finish!: (added: boolean) => void;
    const onAddContact = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
    renderDirectory({ onAddContact });
    await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.add_friend" }));
    const dialog = screen.getByRole("dialog");
    await userEvent.click(within(dialog).getByRole("button", { name: "Writer" }));
    const form = dialog.querySelector("form")!;
    act(() => { fireEvent.submit(form); fireEvent.submit(form); });
    expect(onAddContact).toHaveBeenCalledOnce();
    expect(onAddContact).toHaveBeenCalledWith("agent-writer", "");
    expect((within(dialog).getByRole("searchbox") as HTMLInputElement).disabled).toBe(true);
    expect(within(dialog).getByRole("button", { name: "Writer" }).getAttribute("aria-disabled")).toBe("true");
    for (const button of within(dialog).getAllByRole("button").filter((node) => node.tagName === "BUTTON")) {
      expect((button as HTMLButtonElement).disabled).toBe(true);
    }
    await userEvent.keyboard("{Escape}");
    expect(screen.getByRole("dialog")).toBe(dialog);
    await act(async () => finish(false));
    expect(screen.getByRole("dialog")).toBe(dialog);
    expect((within(dialog).getByRole("button", { name: "agent_options.contact.add_friend" }) as HTMLButtonElement).disabled).toBe(false);
  });

  it("rejects a selected candidate that leaves the current eligible directory", async () => {
    const { props, rerenderDirectory } = renderDirectory();
    await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.add_friend" }));
    await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Writer" }));
    rerenderDirectory({ agents: [CURRENT_AGENT] });
    const dialog = screen.getByRole("dialog");
    expect((within(dialog).getByRole("button", { name: "agent_options.contact.add_friend" }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.submit(dialog.querySelector("form")!);
    expect(props.onAddContact).not.toHaveBeenCalled();
  });

  it("does not close another Agent's newly opened dialog after a late successful add", async () => {
    let finish!: (added: boolean) => void;
    const onAddContact = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
    const { rerenderDirectory } = renderDirectory({ onAddContact });
    await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.add_friend" }));
    await userEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Writer" }));
    fireEvent.submit(screen.getByRole("dialog").querySelector("form")!);
    rerenderDirectory({ agent: { ...CURRENT_AGENT, agent_id: "other" } });
    expect(screen.queryByRole("dialog")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "agent_options.contact.add_friend" }));
    const nextDialog = screen.getByRole("dialog");
    await act(async () => finish(true));
    expect(screen.getByRole("dialog")).toBe(nextDialog);
  });
});
