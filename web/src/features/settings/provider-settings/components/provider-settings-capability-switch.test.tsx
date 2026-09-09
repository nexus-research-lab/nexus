// INPUT: 模型能力名称、选中状态与变更回调。
// OUTPUT: 名称点击与键盘都只激活对应的原生开关一次。
// POS: 模型能力行 DOM 回归，不涉及模型持久化。
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { CapabilitySwitch } from "./provider-settings-capability-switch";

it("activates only the labelled capability through text and keyboard", async () => {
  const user = userEvent.setup();
  const onVision = vi.fn();
  const onTools = vi.fn();
  render(<>
    <CapabilitySwitch label="视觉理解与图像内容识别" checked={false} onChange={onVision} />
    <CapabilitySwitch label="工具调用" checked onChange={onTools} />
  </>);
  await user.click(screen.getByText("视觉理解与图像内容识别"));
  expect(onVision).toHaveBeenCalledExactlyOnceWith(true);
  expect(onTools).not.toHaveBeenCalled();
  screen.getByRole("switch", { name: "工具调用" }).focus();
  await user.keyboard(" ");
  expect(onTools).toHaveBeenCalledExactlyOnceWith(false);
});
