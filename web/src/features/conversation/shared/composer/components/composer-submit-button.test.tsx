// INPUT: Composer 发送/停止状态、对应动作和可访问文案。
// OUTPUT: 证明共享 Button 承载下仍只触发当前投影动作，并正确禁用准备态。
// POS: Composer 提交动作行为测试；不重复验证 shared UiButton 的基础 DOM 合同。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  ComposerSubmitButton,
  type ComposerSubmitButtonProps,
} from "./composer-submit-button";

function renderSubmitButton(
  patch: Partial<ComposerSubmitButtonProps> = {},
) {
  const props: ComposerSubmitButtonProps = {
    isDisabled: false,
    isGoalCreating: false,
    isGoalMode: false,
    isPreparingAttachments: false,
    onSend: vi.fn(),
    onStop: vi.fn(),
    sendLabel: "发送",
    shouldStop: false,
    stopLabel: "停止",
    ...patch,
  };
  render(<ComposerSubmitButton {...props} />);
  return props;
}

describe("ComposerSubmitButton", () => {
  it("offers an icon-only continuation action", async () => {
    const user = userEvent.setup();
    const props = renderSubmitButton({ shouldContinue: true, sendLabel: "Continue" });
    const button = screen.getByRole("button", { name: "Continue" });
    expect(button.textContent).toBe("");
    await user.click(button);
    expect(props.onSend).toHaveBeenCalledOnce();
    expect(props.onStop).not.toHaveBeenCalled();
  });

  it("keeps stopping available while send is disabled or another preparation flag remains", async () => {
    const user = userEvent.setup();
    const props = renderSubmitButton({ shouldStop: true, isDisabled: true, isGoalCreating: true, isPreparingAttachments: true });
    const stop = screen.getByRole("button", { name: "停止" });
    expect((stop as HTMLButtonElement).disabled).toBe(false);
    await user.click(stop);
    expect(props.onStop).toHaveBeenCalledOnce();
    expect(props.onSend).not.toHaveBeenCalled();
  });
});
