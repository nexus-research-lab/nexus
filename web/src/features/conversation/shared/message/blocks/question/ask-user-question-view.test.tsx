// INPUT: 结构化问题、回答草稿、交互状态与提交/拒绝命令。
// OUTPUT: 证明问答视图复用共享动作/排版，同时保持选项、自定义回答和终态行为。
// POS: AskUserQuestion 纯视图 DOM 合同；草稿原子更新与异步提交事务由 model/controller 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { AskUserQuestionView } from "./ask-user-question-view";

const QUESTION = {
  header: "研究口径",
  multi_select: false,
  options: [{
    description: "先保证结论稳健",
    label: "保守",
  }],
  question: "这次分析采用哪种研究口径？",
};

function renderView(
  overrides: Partial<Parameters<typeof AskUserQuestionView>[0]> = {},
) {
  const props: Parameters<typeof AskUserQuestionView>[0] = {
    answerSummary: "",
    draft: [{ customAnswer: "", selectedOptions: new Set<string>() }],
    draftComplete: false,
    expanded: true,
    isReady: true,
    isSubmitting: false,
    onDeny: vi.fn(),
    onSubmit: vi.fn(),
    onToggleOption: vi.fn(),
    onUpdateCustomAnswer: vi.fn(),
    questions: [QUESTION],
    readOnly: false,
    status: "active",
    submitEnabled: true,
    ...overrides,
  };
  const result = render(
    <I18N_CONTEXT.Provider
      value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
    >
      <AskUserQuestionView {...props} />
    </I18N_CONTEXT.Provider>,
  );
  return { ...result, props };
}

describe("AskUserQuestionView", () => {
  it("uses shared decision actions without changing question input behavior", async () => {
    const user = userEvent.setup();
    const { props } = renderView();
    const parentClick = vi.fn();
    document.body.addEventListener("click", parentClick);

    const deny = screen.getByRole("button", { name: "composer.permission_deny" });
    const submit = screen.getByRole("button", { name: "composer.question_submit" });
    expect(deny.className).toContain("radius-control-sm");
    expect(submit.className).toContain("bg-(--button-primary-background)");
    expect(deny.getAttribute("type")).toBe("button");
    expect(submit.getAttribute("type")).toBe("button");

    await user.click(deny);
    await user.click(submit);
    expect(props.onDeny).toHaveBeenCalledTimes(1);
    expect(props.onSubmit).toHaveBeenCalledTimes(1);
    expect(parentClick).not.toHaveBeenCalled();
    document.body.removeEventListener("click", parentClick);

    await user.click(screen.getByText("保守"));
    expect(props.onToggleOption).toHaveBeenCalledWith(0, "保守");

    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "补充回答" },
    });
    expect(props.onUpdateCustomAnswer).toHaveBeenCalledWith(0, "补充回答");
    expect(screen.getByText(QUESTION.question).className).toContain("ui-type-body");
    expect(screen.getByText(QUESTION.options[0].description).className)
      .toContain("ui-type-metadata");
  });

  it("uses semantic typography for the collapsed terminal resolution", () => {
    renderView({
      answerSummary: "保守",
      expanded: false,
      onDeny: undefined,
      status: "submitted",
      submitEnabled: false,
    });

    expect(screen.getByText("composer.question_status_submitted").className)
      .toContain("ui-type-caption");
    expect(screen.getByText("保守").className).toContain("ui-type-caption");
    expect(screen.queryByRole("button", { name: "composer.question_submit" }))
      .toBeNull();
  });
});
