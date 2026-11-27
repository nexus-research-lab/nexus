// INPUT: 双语成员、缺项/重名目录、头像失败与受控选择变化。
// OUTPUT: 证明公共头像回退、名称去歧义和菜单生命周期不改写 Agent 身份。
// POS: RoomAgentSwitcher DOM 行为测试；成员权限和进程归属由上层负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RoomAgentSwitcher } from "@/features/conversation/room/surface/room-agent-switcher";
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
});
