// INPUT: 同名 Agent 选择、目录缺项/恢复与已有外部对象草稿。
// OUTPUT: 证明目录变化不能自动改绑第一项，恢复后仍提交用户选择的精确 ID。
// POS: 配对创建表单的实际 DOM/提交回归，复用公共 Select 与 Dialog。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import type { Agent } from "@/types/agent/agent";
import { CreatePairingDialog } from "./pairing-create-dialog";

const a: Agent = { agent_id: "agent-a", name: "Nova", created_at: 1, options: {}, status: "idle", workspace_path: "/a" };
const b: Agent = { ...a, agent_id: "agent-b", created_at: 2, workspace_path: "/b" };
const t: I18nContextValue["t"] = (key, params) => Object.entries(params ?? {}).reduce((text, [name, value]) => text.replaceAll(`{${name}}`, String(value)), MESSAGES.zh[key]);

describe("pairing creation identity", () => {

  it.each([{ remaining: [] }, { remaining: [a] }])("does not replace the selected Agent when the directory becomes $remaining", async ({ remaining }) => {
    const user = userEvent.setup();
    const onCreate = vi.fn(async () => false);
    const onClose = vi.fn();
    const view = (agents: Agent[]) => (
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
        <CreatePairingDialog pairings={[]} onManage={vi.fn()} agents={agents} blocked={false} failure={null} onClose={onClose} onCreate={onCreate} />
      </I18N_CONTEXT.Provider>
    );
    const { rerender } = render(view([a, b]));
    await user.type(screen.getByLabelText(/外部对象 ID/), "chat-42");
    await user.click(screen.getByRole("button", { name: "选择处理智能体" }));
    await user.click(screen.getByRole("option", { name: "2 · Nova" }));
    rerender(view(remaining));
    expect(screen.getByRole("button", { name: "选择处理智能体" }).textContent).toContain("当前智能体不可用");
    const submit = screen.getByRole("button", { name: "新增配对" });
    expect((submit as HTMLButtonElement).disabled).toBe(true);
    fireEvent.submit(submit.closest("form")!);
    expect(onCreate).not.toHaveBeenCalled();

    rerender(view([b, a]));
    expect(screen.getByRole("button", { name: "选择处理智能体" }).textContent).toContain("2 · Nova");
    await user.click(screen.getByRole("button", { name: "新增配对" }));
    expect(onCreate).toHaveBeenCalledOnce();
    expect(onCreate).toHaveBeenCalledWith(expect.objectContaining({ agent_id: b.agent_id, external_ref: "chat-42" }));
    expect(onClose).not.toHaveBeenCalled();
  });
});

it("opens the selected existing WeChat contact without creating another pairing", async () => {
  const user = userEvent.setup();
  const onManage = vi.fn();
  const onCreate = vi.fn(async () => false);
  const item = {
    pairing_id: "pair-1", channel_type: "weixin-personal", chat_type: "dm",
    account_id: "account-1", external_ref: "contact-42", external_name: "小李",
    agent_id: a.agent_id, session_key: "session", status: "pending", source: "ingress",
    created_at: "", updated_at: "",
  } as const;
  render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
    <CreatePairingDialog pairings={[item]} onManage={onManage} agents={[a]} blocked={false} failure={null} onCreate={onCreate} onClose={vi.fn()} />
  </I18N_CONTEXT.Provider>);
  await user.click(screen.getByRole("button", { name: t("capability.pairing_select_chat_type") }));
  await user.click(screen.getByRole("option", { name: "群聊" }));
  await user.click(screen.getByRole("button", { name: t("capability.pairing_select_channel") }));
  await user.click(screen.getByRole("option", { name: "微信" }));
  expect(screen.getByRole("button", { name: t("capability.pairing_select_chat_type") }).textContent).toContain(t("capability.pairing_chat_dm"));
  await user.click(screen.getByRole("button", { name: "选择联系人或群聊" }));
  await user.click(screen.getByRole("option", { name: /小李.*account-1/ }));
  await user.click(screen.getByRole("button", { name: "管理已有配对" }));
  expect(onManage).toHaveBeenCalledWith(item);
  expect(onCreate).not.toHaveBeenCalled();
});
