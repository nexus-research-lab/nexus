// INPUT: A local Agent directory with distinct business tags, providers and permissions.
// OUTPUT: Search/create stay in the header, filters/view only appear for larger directories, and exact commands survive filtering.
// POS: Contacts page DOM regression; no directory API, runtime, persistence or geometry assertions.

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { Agent } from "@/types/agent/agent";
import { ContactsDirectory } from "./contacts-directory";

const agents: Agent[] = [
  { agent_id: "research", name: "Researcher", business_tags: ["Research"], avatar: null,
    created_at: 1, description: "Research reports", status: "idle", workspace_path: "/research",
    options: { permission_mode: "acceptEdits", provider: "anthropic" } },
  { agent_id: "writer", name: "Writer", business_tags: ["Writing"], avatar: null,
    created_at: 2, description: "Editorial work", status: "idle", workspace_path: "/writing",
    options: { permission_mode: "default" } },
  { agent_id: "operations", name: "Operations", business_tags: ["Ops"], vibe_tags: ["Research"], avatar: null,
    created_at: 3, description: "Service operations", status: "idle", workspace_path: "/operations",
    options: { permission_mode: "default", provider: "anthropic" } },
];

// 筛选只在目录达到阈值后出现；补足无标签、跟随默认的 Agent 以便测试真实筛选。
const fillers: Agent[] = Array.from({ length: 5 }, (_, index) => ({
  agent_id: `filler-${index}`, name: `Filler ${index}`, avatar: null, created_at: 10 + index,
  description: "", status: "idle", workspace_path: `/filler-${index}`, options: { permission_mode: "default" },
}));
const largeDirectory = [...agents, ...fillers];

function directory(items = largeDirectory) {
  const actions = { onCreateAgent: vi.fn(), onOpenAgent: vi.fn(), onOpenDirectRoom: vi.fn() };
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <ContactsDirectory agents={items} {...actions} />
  </I18N_CONTEXT.Provider>);
  return actions;
}

describe("ContactsDirectory", () => {
  it("owns one search/create entry and keeps exact commands after a business-tag filter and view switch", async () => {
    const user = userEvent.setup();
    const actions = directory();
    expect(screen.getAllByRole("searchbox")).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "contacts.new_agent" })).toHaveLength(1);
    expect(screen.getAllByRole("article")).toHaveLength(8);
    await user.click(screen.getByRole("button", { name: "contacts.filters.tags" }));
    await user.click(screen.getByRole("option", { name: "Research" }));
    expect(screen.getAllByRole("article")).toHaveLength(1);
    expect(screen.getByRole("heading", { name: "Researcher" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "contacts.views.list" }));
    expect(screen.queryByRole("article")).toBeNull();
    await user.click(screen.getByRole("button", { name: "contacts.chat Researcher" }));
    expect(actions.onOpenAgent).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "common.edit Researcher" }));
    await user.click(screen.getByRole("button", { name: "contacts.new_agent" }));
    expect(actions.onOpenAgent).toHaveBeenCalledExactlyOnceWith("research");
    expect(actions.onOpenDirectRoom).toHaveBeenCalledExactlyOnceWith("research");
    expect(actions.onCreateAgent).toHaveBeenCalledOnce();
  });

  it("combines all filters with search and clears them together while retaining the selected view", async () => {
    const user = userEvent.setup();
    directory();
    await user.click(screen.getByRole("button", { name: "contacts.filters.tags" }));
    await user.click(screen.getByRole("option", { name: "Research" }));
    await user.click(screen.getByRole("button", { name: "contacts.filters.permissions" }));
    await user.click(screen.getByRole("option", { name: "agent_options.advanced.permission.accept_edits.label" }));
    await user.type(screen.getByRole("searchbox"), "research");
    expect(screen.getAllByRole("article")).toHaveLength(1);
    await user.click(screen.getByRole("button", { name: "contacts.filters.providers" }));
    await user.click(screen.getByRole("option", { name: "agent_options.identity.follow_default_provider" }));
    const empty = screen.getByRole("status");
    expect(within(empty).getByText("contacts.no_matches")).toBeTruthy();
    expect(screen.queryByRole("article")).toBeNull();
    expect(screen.getByRole("button", { name: "contacts.new_agent" })).toBeTruthy();
    await user.click(screen.getByRole("button", { name: "contacts.views.list" }));
    await user.click(screen.getByRole("button", { name: "state.clear_filters" }));
    expect(screen.queryByRole("status")).toBeNull();
    expect((screen.getByRole("searchbox") as HTMLInputElement).value).toBe("");
    expect(screen.getByRole("button", { name: "contacts.views.list" }).getAttribute("aria-pressed")).toBe("true");
    for (const name of ["Researcher", "Writer", "Operations"]) expect(screen.getByRole("heading", { name })).toBeTruthy();
    for (const name of ["contacts.filters.all_tags", "contacts.filters.all_providers", "contacts.filters.all_permissions"]) {
      expect(screen.getByText(name)).toBeTruthy();
    }
  });

  it("keeps creation available in an empty directory without inventing a filtering failure", async () => {
    const user = userEvent.setup();
    const actions = directory([]);
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.queryByRole("button", { name: "contacts.filters.tags" })).toBeNull();
    await user.click(screen.getByRole("button", { name: "contacts.new_agent" }));
    expect(actions.onCreateAgent).toHaveBeenCalledOnce();
  });
});
