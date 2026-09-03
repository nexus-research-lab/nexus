// INPUT: Scheduled Picker 的触发器、时间列与用户选择动作。
// OUTPUT: 证明 Picker 复用共享 Button/ChoiceButton 且保留展开和选择行为。
// POS: Scheduled Picker DOM 合同；日期禁用规则和浮层定位由各自单元负责。

import { createRef } from "react";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Clock3 } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { PickerTrigger } from "./picker-trigger";
import { TimePickerColumn } from "./time-picker-column";

describe("Scheduled Picker controls", () => {
  it("uses the shared button trigger and exposes its popup state", async () => {
    const onToggle = vi.fn();
    const user = userEvent.setup();
    render(
      <PickerTrigger
        anchorRef={createRef<HTMLButtonElement>()}
        display="09:30"
        icon={Clock3}
        isOpen
        label="调度"
        onToggle={onToggle}
      />,
    );

    const trigger = screen.getByRole("button", { name: "调度: 09:30" });
    expect(trigger.className).toContain("radius-control-lg");
    expect(trigger.className).toContain("ui-type-control");
    expect(trigger.getAttribute("aria-expanded")).toBe("true");
    expect(trigger.getAttribute("aria-haspopup")).toBe("dialog");
    await user.click(trigger);
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("uses shared choice buttons and dispatches the selected time value", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(
      <TimePickerColumn
        onSelect={onSelect}
        options={["08", "09"]}
        value="08"
      />,
    );

    const selected = screen.getByRole("button", { name: "08" });
    expect(selected.className).toContain("radius-control-md");
    expect(selected.getAttribute("aria-pressed")).toBe("true");
    await user.click(screen.getByRole("button", { name: "09" }));
    expect(onSelect).toHaveBeenCalledWith("09");
  });
});
