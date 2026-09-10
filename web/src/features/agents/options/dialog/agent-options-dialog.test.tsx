// INPUT: 同时存在的 Agent 编辑弹窗。
// OUTPUT: 每个弹窗的 accessible name 只关联自己标题。
// POS: 模态壳层身份回归，编辑器事务由原模块测试负责。
import { render } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { AgentOptionsDialog } from "./agent-options-dialog";
vi.mock("@/features/agents/options/agent-options-editor", () => ({ AgentOptionsDialogEditor: () => null }));
it("isolates title references across dialog instances", () => {
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    {["First", "Second"].map((name) => <AgentOptionsDialog key={name} onClose={vi.fn()} onDelete={vi.fn()} onSave={vi.fn()} onValidateName={vi.fn()}
      state={{ kind: "edit", agentId: name, isMain: false, initial: { avatar: "", businessTags: [], description: "", options: {}, title: name, vibeTags: [] } }} />)}
  </I18N_CONTEXT.Provider>);
  const dialogs = [...document.querySelectorAll('[role="dialog"]')];
  expect(dialogs).toHaveLength(2);
  const labels = dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"));
  expect(new Set(labels).size).toBe(2);
  for (const [index, dialog] of dialogs.entries()) {
    const title = document.getElementById(labels[index]!);
    expect(dialog.contains(title)).toBe(true);
    expect(title?.textContent).toBe(index === 0 ? "First" : "Second");
  }
});
