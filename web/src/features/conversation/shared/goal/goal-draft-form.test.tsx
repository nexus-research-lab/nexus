// INPUT: Goal 草稿、完整预算文本、双语文案与稳定回调。
// OUTPUT: 证明字段/弹窗命名独立、错误关联准确、非法/未知结果不可提交，并复用 Spinner。
// POS: Goal 编辑表单 DOM 合同；提交可用性由 goal-model 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { GoalDraftForm } from "./goal-draft-form";
import { goalTestI18n } from "./goal.test-support";

describe("GoalDraftForm", () => {
  it("uses the shared medium Spinner while submitting", () => {
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={goalTestI18n("zh")}
      >
        <GoalDraftForm
          budget=""
          disabled={false}
          isLoading
          loadingLabel="保存中"
          mutationBlocked={false}
          objective="统一前端规范"
          onBudgetChange={vi.fn()}
          onCancel={vi.fn()}
          onObjectiveChange={vi.fn()}
          onRefresh={vi.fn()}
          onSubmit={vi.fn()}
          reliability={null}
        />
      </I18N_CONTEXT.Provider>,
    );

    expect(screen.getByRole("dialog")).toBeTruthy();
    const spinner = container.ownerDocument.querySelector("svg.animate-spin");
    expect(spinner?.getAttribute("class")).toContain("h-4 w-4");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });
});

const FORM_PROPS = {
  budget: "100", disabled: false, isLoading: false, mutationBlocked: false,
  objective: "Objective", reliability: null,
  onBudgetChange: vi.fn(), onCancel: vi.fn(), onObjectiveChange: vi.fn(),
  onRefresh: vi.fn(), onSubmit: vi.fn(),
};

describe("Goal draft validation and field identity", () => {
  it.each(["zh", "en"] as const)("uses %s labels and associates the complete budget error with the input", (locale) => {
    const context = goalTestI18n(locale);
    const onSubmit = vi.fn();
    render(<I18N_CONTEXT.Provider value={context}>
      <GoalDraftForm {...FORM_PROPS} budget="100abc" onSubmit={onSubmit} />
    </I18N_CONTEXT.Provider>);
    const input = screen.getByRole("textbox", { name: context.t("goal.budget_label") }) as HTMLInputElement;
    expect(input.value).toBe("100abc");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    const error = document.getElementById(input.getAttribute("aria-errormessage")!);
    expect(error?.textContent).toBe(context.t("goal.budget_invalid"));
    expect((screen.getByRole("button", { name: context.t("common.save") }) as HTMLButtonElement).disabled).toBe(true);
    fireEvent.submit(input.form!);
    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: context.t("goal.edit_title") })).toBeTruthy();
  });
  it("gives simultaneous forms independent labels and dialog titles", () => {
    render(<I18N_CONTEXT.Provider value={goalTestI18n("en")}>
      <GoalDraftForm {...FORM_PROPS} />
      <GoalDraftForm {...FORM_PROPS} objective="Second objective" />
    </I18N_CONTEXT.Provider>);
    const inputs = Array.from(document.querySelectorAll("input,textarea"));
    const ids = inputs.map((input) => input.id);
    expect(new Set(ids).size).toBe(4);
    for (const input of inputs) {
      const label = Array.from(document.querySelectorAll("label")).find((item) => item.htmlFor === input.id);
      expect(label).toBeTruthy();
      expect(label?.closest("form")).toBe(input.closest("form"));
    }
    const dialogs = Array.from(document.querySelectorAll('[role="dialog"]'));
    const titleIds = dialogs.map((dialog) => dialog.getAttribute("aria-labelledby"));
    expect(new Set(titleIds).size).toBe(2);
    expect(titleIds.every((id) => id && document.getElementById(id)?.textContent === "Edit Goal")).toBe(true);
  });
  it("keeps an unknown-result draft readable and cancellable while blocking further submissions", () => {
    const onCancel = vi.fn();
    const onSubmit = vi.fn();
    render(<I18N_CONTEXT.Provider value={goalTestI18n("en")}>
      <GoalDraftForm {...FORM_PROPS} mutationBlocked onCancel={onCancel} onSubmit={onSubmit} />
    </I18N_CONTEXT.Provider>);
    const input = screen.getByRole("textbox", { name: "Token budget" }) as HTMLInputElement;
    expect(input.disabled).toBe(true);
    expect(input.value).toBe("100");
    fireEvent.submit(input.form!);
    expect(onSubmit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onCancel).toHaveBeenCalledOnce();
  });
});
