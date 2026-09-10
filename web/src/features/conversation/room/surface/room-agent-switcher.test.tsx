// INPUT: 双语成员、缺项/重名目录、头像失败与受控选择变化。
// OUTPUT: 证明公共头像回退、名称去歧义和菜单生命周期不改写 Agent 身份。
// POS: RoomAgentSwitcher DOM 行为测试；成员权限和进程归属由上层负责。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RoomAgentSwitcher } from "@/features/conversation/room/surface/room-agent-switcher";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";

const MEMBERS: Agent[] = [
  {
    agent_id: "alpha",
    created_at: 1,
    name: "Alpha",
    options: {},
    status: "idle",
    workspace_path: "/workspace/alpha",
  },
  {
    agent_id: "beta",
    created_at: 2,
    name: "Beta",
    options: {},
    status: "idle",
    workspace_path: "/workspace/beta",
  },
];

function localizedSwitcher(
  props: Parameters<typeof RoomAgentSwitcher>[0],
  locale: Locale = "zh",
) {
  const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {})
    .reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES[locale][key]);
  return (
    <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
      <RoomAgentSwitcher {...props} />
    </I18N_CONTEXT.Provider>
  );
}

describe("RoomAgentSwitcher", () => {
  it.each(["zh", "en"] as const)("keeps duplicate and unnamed identities distinct in %s", async (locale) => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const members = [
      { ...MEMBERS[1], name: " Nova " },
      { ...MEMBERS[0], name: "Nova" },
      { ...MEMBERS[0], agent_id: "internal-empty-agent", name: "  " },
    ];
    const props = { members, onSelect, selectedId: "beta" };
    const { rerender } = render(localizedSwitcher(props, locale));
    const trigger = screen.getByRole("button");
    expect(trigger.getAttribute("title")).toBeNull();
    await user.click(trigger);
    const items = screen.getAllByRole("menuitem");
    expect(items[0].textContent).toContain("2 · Nova");
    expect(items[1].textContent).toContain("1 · Nova");
    const unnamedLabel = locale === "zh" ? "1 · 智能体" : "1 · Agent";
    expect(screen.getByRole("menuitem", { name: unnamedLabel })).toBe(items[2]);
    expect(screen.queryAllByRole("img")).toHaveLength(0);
    expect(document.body.textContent).not.toContain("internal-empty-agent");
    rerender(localizedSwitcher({ ...props, members: [...members].reverse() }, locale));
    expect(screen.getByRole("menu")).toBeTruthy();
    expect(screen.getAllByRole("menuitem")[2].textContent).toContain("2 · Nova");
    await user.click(screen.getByRole("menuitem", { name: unnamedLabel }));
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("internal-empty-agent");
  });

  it("uses the full directory for stable labels without adding candidates", async () => {
    const user = userEvent.setup();
    const directory = MEMBERS.map((member) => ({ ...member, name: "Nova" }));
    render(localizedSwitcher({ directory, members: [directory[1]], onSelect: vi.fn(), selectedId: "beta", variant: "task" }));
    const trigger = screen.getByRole("button");
    expect(trigger.getAttribute("title")).toBeNull();
    await user.click(trigger);
    expect(screen.getAllByRole("menuitem")).toHaveLength(1);
    expect(screen.getByRole("menuitem", { name: "2 · Nova" })).toBeTruthy();
    expect(screen.queryByRole("menuitem", { name: "1 · Nova" })).toBeNull();
  });

  it.each(["zh", "en"] as const)("represents a missing selection without selecting the first member in %s", async (locale) => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const props = { members: MEMBERS, onSelect, selectedId: "missing-agent-internal" };
    const { rerender } = render(localizedSwitcher(props, locale));
    const unavailable = MESSAGES[locale]["agent.selection_unavailable"];
    const trigger = screen.getByRole("button");
    expect(trigger.getAttribute("title")).toBeNull();
    expect(trigger.textContent).not.toContain("Alpha");
    expect(onSelect).not.toHaveBeenCalled();
    await user.click(trigger);
    const missingItem = screen.getByRole("menuitem", { name: unavailable });
    expect(missingItem.getAttribute("aria-disabled") === "true" || missingItem.hasAttribute("disabled")).toBe(true);
    await user.click(missingItem);
    expect(onSelect).not.toHaveBeenCalled();
    expect(document.body.textContent).not.toContain(props.selectedId);
    await user.click(screen.getByRole("menuitem", { name: "Beta" }));
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("beta");
    rerender(localizedSwitcher({ ...props, selectedId: "beta" }, locale));
    expect(trigger.getAttribute("title")).toBeNull();
  });

  it("discards an open menu when candidates disappear and does not reopen on recovery", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const props = { members: MEMBERS, onSelect, selectedId: "alpha" };
    const { rerender } = render(localizedSwitcher(props));
    const trigger = screen.getByRole("button");
    await user.click(trigger);
    rerender(localizedSwitcher({ ...props, members: [] }));
    expect(screen.queryByRole("menu")).toBeNull();
    expect(trigger.hasAttribute("disabled")).toBe(true);
    expect(trigger.getAttribute("title")).toBeNull();
    expect(onSelect).not.toHaveBeenCalled();
    rerender(localizedSwitcher(props));
    expect(trigger.getAttribute("aria-expanded")).toBe("false");
    expect(trigger.hasAttribute("disabled")).toBe(false);
    await user.click(trigger);
    rerender(localizedSwitcher({ ...props, members: [MEMBERS[0]] }));
    expect(screen.queryByRole("menu")).toBeNull();
  });

  it("closes on external selection changes and restores trigger focus after keyboard selection", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const props = { members: MEMBERS, onSelect, selectedId: "alpha" };
    const { rerender } = render(localizedSwitcher(props));
    const trigger = screen.getByRole("button");
    trigger.focus();
    await user.keyboard("{Enter}");
    rerender(localizedSwitcher({ ...props, selectedId: "beta" }));
    expect(screen.queryByRole("menu")).toBeNull();
    expect(trigger.getAttribute("title")).toBeNull();
    expect(onSelect).not.toHaveBeenCalled();
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(document.activeElement).toBe(screen.getByRole("menuitem", { name: "Alpha" }));
    await user.keyboard("{ArrowDown}{Enter}");
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("beta");
    expect(document.activeElement).toBe(trigger);
  });

  it("uses shared failed-image recovery with decorative complete-character initials", async () => {
    const user = userEvent.setup();
    const member = { ...MEMBERS[0], name: "👩‍💻 Nova", avatar: "/broken-agent.png" };
    const props = { members: [member], selectedId: member.agent_id, onSelect: vi.fn() };
    const { rerender } = render(localizedSwitcher(props));
    const trigger = screen.getByRole("button");
    fireEvent.error(trigger.querySelector("img")!);
    expect(trigger.querySelector('[aria-hidden="true"]')?.textContent).toBe("👩‍💻");
    expect(trigger.querySelector("img")).toBeNull();
    rerender(localizedSwitcher({ ...props, members: [{ ...member, name: "Nora Smith" }] }));
    expect(trigger.querySelector('[aria-hidden="true"]')?.textContent).toBe("N");
    expect(trigger.querySelector("img")).toBeNull();
    rerender(localizedSwitcher({ ...props, members: [{ ...member, avatar: "/changed-agent.png" }] }));
    expect(trigger.querySelector("img")?.getAttribute("src")).toBe("/changed-agent.png");
    await user.click(trigger);
    const item = screen.getByRole("menuitem", { name: "👩‍💻 Nova" });
    fireEvent.error(item.querySelector("img")!);
    expect(within(item).queryAllByRole("img")).toHaveLength(0);
    expect(item.querySelector('[aria-hidden="true"]')?.textContent).toBe("👩‍💻");
  });

  it("allows an explicit choice from an unbound selector and hides an entirely empty selector", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const props = { members: MEMBERS, selectedId: "", onSelect };
    const { rerender } = render(localizedSwitcher(props));
    expect(onSelect).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: MESSAGES.zh["room.switch_agent"] }));
    await user.click(screen.getByRole("menuitem", { name: "Alpha" }));
    expect(onSelect).toHaveBeenCalledExactlyOnceWith("alpha");
    rerender(localizedSwitcher({ ...props, members: [] }));
    expect(screen.queryByRole("button")).toBeNull();
  });

  it("uses the shared trigger state and selects from the shared action menu", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <I18nProvider>
        <RoomAgentSwitcher
          members={MEMBERS}
          onSelect={onSelect}
          selectedId="alpha"
        />
      </I18nProvider>,
    );

    const trigger = screen.getByRole("button", { name: /Alpha/ });
    expect(trigger.className).toContain("min-h-7");
    expect(trigger.className).toContain("border-transparent");
    expect(trigger.getAttribute("aria-expanded")).toBe("false");

    await user.click(trigger);
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    await user.click(screen.getByRole("menuitem", { name: /Beta/ }));
    expect(onSelect).toHaveBeenCalledWith("beta");
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
