// INPUT: 双语真实栏目/成员菜单、桌面简介请求及类型化内容边界。
// OUTPUT: 成员移除、重复请求、Room/owner 切换与精确保存目标的 DOM 回归。
// POS: 简介导航装配测试；隔离编辑器、记忆和联络 API，不替代各领域测试。

import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, ReactNode } from "react";
import { expect, it, vi } from "vitest";

import { AGENT_DETAIL_TABS, type AgentDetailTabKey } from "@/features/agents/agent-detail-navigation";
import type { AgentOptionsInlineEditor } from "@/features/agents/options/agent-options-editor";
import type { AgentPrivateDomainView } from "@/features/agents/private-domain/agent-private-domain-view";
import type { AgentMemoryView } from "@/features/memory/agent-memory-view";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { Agent, AgentIdentityDraft, AgentNameValidationResult, AgentOptions } from "@/types/agent/agent";
import { RoomAgentAboutSurface } from "./room-agent-about-surface";

const boundaries = vi.hoisted(() => ({ editor: vi.fn(), privateDomain: vi.fn() }));
vi.mock("@/features/agents/options/agent-options-editor", () => ({
  AgentOptionsInlineEditor: (props: ComponentProps<typeof AgentOptionsInlineEditor>) => {
    boundaries.editor(props);
    return <div>
      <output aria-label="Editor Agent">{props.source.initial.title}</output>
      <button onClick={() => void props.onSave("Renamed", options, identity)}>Save fixture</button>
      <button onClick={() => void props.onValidateName?.("Renamed")}>Validate fixture</button>
      <button onClick={() => props.onTabChange?.("advanced")}>Tools from editor</button>
    </div>;
  },
}));
vi.mock("@/features/memory/agent-memory-view", () => ({
  AgentMemoryView: ({ agent }: ComponentProps<typeof AgentMemoryView>) => <output aria-label="Memory Agent">{agent.name}</output>,
}));
vi.mock("@/features/agents/private-domain/agent-private-domain-view", () => ({
  AgentPrivateDomainView: (props: ComponentProps<typeof AgentPrivateDomainView>) => {
    boundaries.privateDomain(props);
    return <output aria-label="Contact Agent">{props.agent.name}</output>;
  },
}));
vi.mock("@/shared/lib/react/use-media-query", () => ({ useMediaQuery: () => false }));
vi.mock("@/store/sidebar", () => ({ useSidebarStore: (selector: (state: unknown) => unknown) => selector({
  collapse_wide_panel_for_right_panel: vi.fn(), expand_wide_panel_after_right_panel: vi.fn(),
}) }));

const alpha: Agent = { agent_id: "alpha", name: "Alpha", created_at: 1, options: {}, status: "idle", workspace_path: "/alpha" };
const beta: Agent = { ...alpha, agent_id: "beta", name: "Beta", workspace_path: "/beta" };
const options: AgentOptions = { permission_mode: "default" };
const identity: AgentIdentityDraft = { description: "New description" };
const validation: AgentNameValidationResult = { name: "Renamed", normalized_name: "renamed", is_valid: true, is_available: true };
type Props = ComponentProps<typeof RoomAgentAboutSurface>;
function baseProps(): Props {
  return { agent: alpha, roomId: "room-a", conversationId: "conversation-a", roomMembers: [alpha, beta], isVisible: true,
    onSaveAgentOptions: vi.fn(async () => undefined), onValidateAgentName: vi.fn(async () => validation) };
}
function localized(children: ReactNode, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce(
    (text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key],
  );
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>{children}</I18N_CONTEXT.Provider>;
}
function tab(key: AgentDetailTabKey, locale: Locale = "en") {
  return within(screen.getByRole("group", { name: MESSAGES[locale]["room.agent_panel_tabs"] })).getByRole("button", {
    name: MESSAGES[locale][AGENT_DETAIL_TABS.find((item) => item.key === key)!.labelKey],
  });
}
async function selectAgent(user: ReturnType<typeof userEvent.setup>, name: string) {
  const trigger = screen.getAllByRole("button").find((button) => button.getAttribute("aria-haspopup") === "menu")!;
  await user.click(trigger);
  await user.click(screen.getByRole("menuitem", { name }));
}

it.each(["en", "zh"] as const)("shares named, keyboard-operable Agent sections in %s", async (locale) => {
  const user = userEvent.setup();
  render(localized(<RoomAgentAboutSurface {...baseProps()} />, locale));
  const group = screen.getByRole("group", { name: MESSAGES[locale]["room.agent_panel_tabs"] });
  expect(within(group).getAllByRole("button").map((button) => button.textContent)).toEqual(
    AGENT_DETAIL_TABS.map((item) => MESSAGES[locale][item.labelKey]),
  );
  expect(tab("identity", locale).getAttribute("aria-pressed")).toBe("true");
  tab("memory", locale).focus();
  await user.keyboard("{Enter}");
  expect(tab("memory", locale).getAttribute("aria-pressed")).toBe("true");
  expect(screen.getByLabelText("Memory Agent").textContent).toBe("Alpha");
  expect(screen.queryByLabelText("Editor Agent")).toBeNull();
  await selectAgent(user, "Beta");
  expect(screen.getByLabelText("Memory Agent").textContent).toBe("Beta");
  expect(tab("memory", locale).getAttribute("aria-pressed")).toBe("true");
});

it("uses each explicit request once, including missing targets and repeated same-target requests", async () => {
  const user = userEvent.setup();
  const props = { ...baseProps(), requestedAgentId: "beta", requestedTab: "memory" as const, requestKey: 1 };
  const view = (next: Props) => localized(<RoomAgentAboutSurface {...next} />);
  const rendered = render(view({ ...props, roomMembers: [alpha] }));
  expect(screen.getByLabelText("Memory Agent").textContent).toBe("Alpha");
  rendered.rerender(view(props));
  expect(screen.getByLabelText("Memory Agent").textContent).toBe("Alpha");
  rendered.rerender(view({ ...props, requestKey: 2 }));
  expect(screen.getByLabelText("Memory Agent").textContent).toBe("Beta");
  await user.click(tab("skills"));
  rendered.rerender(view({ ...props, requestKey: 3 }));
  expect(screen.getByLabelText("Memory Agent").textContent).toBe("Beta");
  expect(tab("memory").getAttribute("aria-pressed")).toBe("true");
});

it("routes editor validation and saves to the displayed Agent without automatic writes", async () => {
  const user = userEvent.setup();
  const props = baseProps();
  render(localized(<RoomAgentAboutSurface {...props} />));
  await selectAgent(user, "Beta");
  expect(props.onSaveAgentOptions).not.toHaveBeenCalled();
  expect(props.onValidateAgentName).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Validate fixture" }));
  expect(props.onValidateAgentName).toHaveBeenCalledExactlyOnceWith("Renamed", "beta");
  await user.click(screen.getByRole("button", { name: "Save fixture" }));
  expect(props.onSaveAgentOptions).toHaveBeenCalledExactlyOnceWith("beta", "Renamed", options, identity);
  await user.click(screen.getByRole("button", { name: "Tools from editor" }));
  expect(tab("advanced").getAttribute("aria-pressed")).toBe("true");
  expect(boundaries.editor).toHaveBeenLastCalledWith(expect.objectContaining({
    isActive: true, showDeleteButton: false,
    source: expect.objectContaining({ kind: "edit", agentId: "beta", initial: expect.objectContaining({ title: "Beta" }) }),
  }));
});


it("reads Agent contacts across sessions in DM while keeping group contacts scoped", async () => {
  const user = userEvent.setup();
  const props = {...baseProps(), isDm: true, requestedTab: "private_domain" as const};
  const { rerender } = render(localized(<RoomAgentAboutSurface {...props} />));
  expect(boundaries.privateDomain).toHaveBeenLastCalledWith(expect.objectContaining({roomId: null, conversationId: null}));
  rerender(localized(<RoomAgentAboutSurface {...props} isDm={false} />));
  await user.click(tab("private_domain"));
  expect(boundaries.privateDomain).toHaveBeenLastCalledWith(expect.objectContaining({roomId: "room-a", conversationId: "conversation-a"}));
});
