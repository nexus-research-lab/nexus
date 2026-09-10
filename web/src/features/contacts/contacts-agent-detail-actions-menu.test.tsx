// INPUT: 当前 Agent、目录命令和真实菜单点击/键盘事件。
// OUTPUT: 证明精确身份重置、名称刷新连续及聊天/建群/删除动作互相隔离。
// POS: Contacts 详情窄窗操作回归；删除仍只进入页面确认，不执行 API。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";

import { ContactsAgentDetailActionsMenu } from "./contacts-agent-detail-actions-menu";

beforeEach(() => localStorage.setItem(LOCALE_STORAGE_KEY, "zh"));
const commands = () => ({ onCreateTeam: vi.fn(), onDelete: vi.fn(), onOpenDirectRoom: vi.fn() });

describe("ContactsAgentDetailActionsMenu", () => {
  it("uses localized full action names without leaking an ID for missing names", async () => {
    const user = userEvent.setup();
    localStorage.setItem(LOCALE_STORAGE_KEY, "en");
    const callbacks = commands();
    const view = render(<ContactsAgentDetailActionsMenu agentId="private-agent-id" agentName="Pixel" {...callbacks} />, { wrapper: I18nProvider });
    const trigger = screen.getByRole("button", { name: "Actions for Pixel" });
    view.rerender(<ContactsAgentDetailActionsMenu agentId="private-agent-id" agentName="" {...callbacks} />);
    expect(trigger.getAttribute("aria-label")).not.toContain("private-agent-id");
    expect(trigger.getAttribute("aria-label")).toBe("Actions for Agent");
    await user.click(trigger);
    expect(screen.getByRole("menu", { name: "Actions for Agent" })).toBeTruthy();
  });

  it("dispatches each explicit action once and returns focus to the named trigger", async () => {
    const user = userEvent.setup();
    const callbacks = commands();
    render(<ContactsAgentDetailActionsMenu agentId="a" agentName="Nova" {...callbacks} />, { wrapper: I18nProvider });
    const trigger = screen.getByRole("button", { name: "Nova的操作" });
    await user.click(trigger);
    await user.keyboard("{Enter}");
    expect(callbacks.onOpenDirectRoom).toHaveBeenCalledOnce();
    expect(document.activeElement).toBe(trigger);
    await user.click(trigger);
    await user.keyboard("{ArrowDown}{Enter}");
    expect(callbacks.onCreateTeam).toHaveBeenCalledOnce();
    expect(callbacks.onDelete).not.toHaveBeenCalled();
    await user.click(trigger);
    expect(screen.getAllByRole("separator")).toHaveLength(1);
    await user.keyboard("{End}{Enter}");
    expect(callbacks.onDelete).toHaveBeenCalledOnce();
    expect(screen.queryByRole("menu")).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("closes when identity changes, keeps name updates open and uses only current commands", async () => {
    const user = userEvent.setup();
    const first = commands();
    const second = commands();
    const view = render(<ContactsAgentDetailActionsMenu agentId="a" agentName="Nova" {...first} />, { wrapper: I18nProvider });
    await user.click(screen.getByRole("button", { name: "Nova的操作" }));
    view.rerender(<ContactsAgentDetailActionsMenu agentId="a" agentName="Nova renamed" {...first} />);
    expect(screen.getByRole("menu", { name: "Nova renamed的操作" })).toBeTruthy();
    view.rerender(<ContactsAgentDetailActionsMenu agentId="b" agentName="Pixel" {...second} />);
    expect(screen.queryByRole("menu")).toBeNull();
    view.rerender(<ContactsAgentDetailActionsMenu agentId="a" agentName="Nova" {...first} />);
    expect(screen.queryByRole("menu")).toBeNull();
    view.rerender(<ContactsAgentDetailActionsMenu agentId="b" agentName="Pixel" {...second} />);
    await user.click(screen.getByRole("button", { name: "Pixel的操作" }));
    await user.keyboard("{End}{Enter}");
    expect(second.onDelete).toHaveBeenCalledOnce();
    for (const callback of Object.values(first)) expect(callback).not.toHaveBeenCalled();
  });
});
