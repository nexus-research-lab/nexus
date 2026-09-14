// INPUT: pending 请求身份、后到工具 ID、回答草稿与同步响应结果。
// OUTPUT: 证明唯一请求、草稿隔离、响应异常恢复及标准控件投影。
// POS: Composer 人工介入 DOM 集成回归；不模拟 runtime 持久化或发送成功事实。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import { ComposerInteractionSurface } from "./composer-interaction-surface";

const question = (id: string): PendingPermission => ({ request_id: id, agent_id: id, interaction_mode: "question", tool_name: "AskUserQuestion",
  tool_input: { questions: [{ question: `Question ${id}`, options: [{ label: "Yes" }, { label: "No" }] }] } });
const wrap = (permissions: PendingPermission[], onResponse: (payload: unknown) => boolean) => <I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
  <ComposerInteractionSurface permissions={permissions} onResponse={onResponse} agentNameMap={{ A: "Agent A", B: "Agent B" }} />
</I18N_CONTEXT.Provider>;

describe("Composer interaction request identity", () => {
  it("preserves answers through non-acceptance and late tool identity, then resets for the next request", async () => {
    const user = userEvent.setup();
    const first = question("A");
    const second = question("B");
    const onResponse = vi.fn().mockReturnValueOnce(false).mockReturnValue(true);
    const { rerender } = render(wrap([first, second], onResponse));
    expect(screen.getByText("Agent A")).toBeTruthy();
    expect(screen.queryByText("Question B")).toBeNull();
    await user.click(screen.getByRole("radio", { name: /Yes/ }));
    fireEvent.change(screen.getByRole("textbox"), { target: { value: "Custom answer" } });
    expect((screen.getByRole("radio", { name: /Yes/ }) as HTMLInputElement).checked).toBe(false);
    await user.click(screen.getByRole("button", { name: "composer.question_submit" }));
    expect(onResponse.mock.calls[0][0]).toEqual({ decision: "allow", request_id: "A", user_answers: [{ question_index: 0, selected_options: ["Custom answer"] }] });
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("Custom answer");
    rerender(wrap([{ ...first, tool_use_id: "late-tool-a" }, second], onResponse));
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("Custom answer");
    await user.click(screen.getByRole("button", { name: "composer.question_submit" }));
    expect(onResponse).toHaveBeenCalledTimes(2);
    rerender(wrap([second], onResponse));
    expect(screen.getByText("Agent B")).toBeTruthy();
    expect((screen.getByRole("textbox") as HTMLTextAreaElement).value).toBe("");
    await user.click(screen.getByRole("button", { name: "composer.permission_deny" }));
    expect(onResponse).toHaveBeenLastCalledWith({ decision: "deny", request_id: "B" });
  });

  it("releases the local response guard after an exception without replaying or losing the draft", async () => {
    const user = userEvent.setup();
    const log = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const onResponse = vi.fn().mockImplementationOnce(() => { throw new Error("adapter failure"); }).mockReturnValue(true);
    try {
      render(wrap([question("A")], onResponse));
      await user.click(screen.getByRole("radio", { name: /Yes/ }));
      await user.click(screen.getByRole("button", { name: "composer.question_submit" }));
      expect(onResponse).toHaveBeenCalledOnce();
      expect((screen.getByRole("radio", { name: /Yes/ }) as HTMLInputElement).checked).toBe(true);
      expect((screen.getByRole("button", { name: "composer.question_submit" }) as HTMLButtonElement).disabled).toBe(false);
      await user.click(screen.getByRole("button", { name: "composer.question_submit" }));
      expect(onResponse).toHaveBeenCalledTimes(2);
      expect(log).toHaveBeenCalledOnce();
    } finally { log.mockRestore(); }
  });

  it("keeps radio groups local even when two question surfaces use the same question index", async () => {
    const user = userEvent.setup();
    render(<>{wrap([question("A")], vi.fn())}{wrap([question("B")], vi.fn())}</>);
    const groups = screen.getAllByRole("group");
    const first = within(groups[0]).getByRole("radio", { name: /Yes/ }) as HTMLInputElement;
    const second = within(groups[1]).getByRole("radio", { name: /No/ }) as HTMLInputElement;
    await user.click(first);
    await user.click(second);
    expect(first.name).not.toBe(second.name);
    expect(first.checked).toBe(true);
    expect(second.checked).toBe(true);
  });
});
