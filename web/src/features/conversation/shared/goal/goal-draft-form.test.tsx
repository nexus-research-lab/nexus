// INPUT: 正在提交的 Goal 草稿与稳定的表单回调。
// OUTPUT: 证明提交反馈复用共享 Spinner，且对话框仍保持表单语义。
// POS: Goal 编辑表单 DOM 合同；提交可用性由 goal-model 测试负责。

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { GoalDraftForm } from "./goal-draft-form";

describe("GoalDraftForm", () => {
  it("uses the shared medium Spinner while submitting", () => {
    const { container } = render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
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
