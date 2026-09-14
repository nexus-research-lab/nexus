// INPUT: 同名 Agent 选择、目录缺项/恢复与已有外部对象草稿。
// OUTPUT: 证明目录变化不能自动改绑第一项，恢复后仍提交用户选择的精确 ID。
// POS: 配对创建表单的实际 DOM/提交回归，复用公共 Select 与 Dialog。

import { act, fireEvent, render, screen } from "@testing-library/react";
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
  it("keeps field labels attached to their own dialog instance", () => {
    render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
      {[1, 2].map((key) => <CreatePairingDialog key={key} agents={[a]} blocked={false} failure={null} onClose={vi.fn()} onCreate={vi.fn(async () => false)} />)}
    </I18N_CONTEXT.Provider>);
    const forms = Array.from(document.querySelectorAll("form"));
    expect(forms).toHaveLength(2);
    for (const form of forms) {
      for (const label of form.querySelectorAll<HTMLLabelElement>("label[for]")) {
        expect(label.control?.closest("form")).toBe(form);
      }
    }
    const dialogLabels = Array.from(document.querySelectorAll('[role="dialog"]'), (dialog) => dialog.getAttribute("aria-labelledby"));
    expect(new Set(dialogLabels).size).toBe(2);
  });

  it.each([{ remaining: [] }, { remaining: [a] }])("does not replace the selected Agent when the directory becomes $remaining", async ({ remaining }) => {
    const user = userEvent.setup();
    const onCreate = vi.fn(async () => false);
    const onClose = vi.fn();
    const view = (agents: Agent[]) => (
      <I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}>
        <CreatePairingDialog agents={agents} blocked={false} failure={null} onClose={onClose} onCreate={onCreate} />
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


it("cannot dismiss or edit the creation draft while its submission is pending", async () => {
  let finish!: (created: boolean) => void;
  const onCreate = vi.fn(() => new Promise<boolean>((resolve) => { finish = resolve; }));
  const onClose = vi.fn();
  const user = userEvent.setup();
  render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t }}><CreatePairingDialog agents={[a]} blocked={false} failure={null} onCreate={onCreate} onClose={onClose} /></I18N_CONTEXT.Provider>);
  await user.type(screen.getByLabelText(/外部对象 ID/), "chat-42");
  await user.click(screen.getByRole("button", { name: "新增配对" }));
  expect((screen.getByLabelText(/外部对象 ID/) as HTMLInputElement).disabled).toBe(true);
  await user.keyboard("{Escape}");
  expect(onClose).not.toHaveBeenCalled();
  await act(async () => finish(false));
  expect((screen.getByLabelText(/外部对象 ID/) as HTMLInputElement).disabled).toBe(false);
  expect((screen.getByLabelText(/外部对象 ID/) as HTMLInputElement).value).toBe("chat-42");
  await user.keyboard("{Escape}");
  expect(onClose).toHaveBeenCalledOnce();
});


it("switches to English without losing the pairing draft or changing protocol values", async () => {
  const user = userEvent.setup();
  const onCreate = vi.fn(async () => false);
  const view = (locale: "en" | "zh") => <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key] }}><CreatePairingDialog agents={[a]} blocked={false} failure={null} onCreate={onCreate} onClose={vi.fn()} /></I18N_CONTEXT.Provider>;
  const { rerender } = render(view("zh"));
  await user.type(screen.getByLabelText(/外部对象 ID/), "chat-42");
  rerender(view("en"));
  expect((screen.getByLabelText(/External contact ID/) as HTMLInputElement).value).toBe("chat-42");
  await user.click(screen.getByRole("button", { name: "Select IM channel" }));
  await user.click(screen.getByRole("option", { name: "WeCom" }));
  await user.click(screen.getByRole("button", { name: "New pairing" }));
  expect(onCreate).toHaveBeenCalledWith(expect.objectContaining({ channel_type: "wechat", external_ref: "chat-42", agent_id: a.agent_id }));
});
