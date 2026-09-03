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
  it("routes the ordinary state only to the send action", async () => {
    const user = userEvent.setup();
    const props = renderSubmitButton();

    await user.click(screen.getByRole("button", { name: "发送" }));

    expect(props.onSend).toHaveBeenCalledTimes(1);
    expect(props.onStop).not.toHaveBeenCalled();
  });

  it("routes an active round only to the stop action", async () => {
    const user = userEvent.setup();
    const props = renderSubmitButton({ shouldStop: true });

    await user.click(screen.getByRole("button", { name: "停止" }));

    expect(props.onStop).toHaveBeenCalledTimes(1);
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it("does not expose a stop action when no stop command exists", async () => {
    const user = userEvent.setup();
    const props = renderSubmitButton({ onStop: undefined, shouldStop: true });

    await user.click(screen.getByRole("button", { name: "发送" }));

    expect(props.onSend).toHaveBeenCalledTimes(1);
  });

  it("honors the controller's explicit disabled send state", () => {
    renderSubmitButton({ isDisabled: true });

    expect(
      (screen.getByRole("button", { name: "发送" }) as HTMLButtonElement).disabled,
    ).toBe(true);
  });
});
