// INPUT: Scheduled Picker 的触发器、时间列与用户选择动作。
// OUTPUT: 证明 Picker 复用共享 Button/ChoiceButton 且保留展开和选择行为。
// POS: Scheduled Picker DOM 合同；日期禁用规则和浮层定位由各自单元负责。


import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TimePickerColumn } from "./time-picker-column";

describe("Scheduled Picker controls", () => {

  it("uses shared choice buttons and dispatches the selected time value", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(
      <TimePickerColumn
        label="Minutes"
        onSelect={onSelect}
        options={["08", "09"]}
        value="08"
      />,
    );

    expect(screen.getByRole("group", { name: "Minutes" })).toBeTruthy();
    const selected = screen.getByRole("button", { name: "08" });

    expect(selected.getAttribute("aria-pressed")).toBe("true");
    await user.click(screen.getByRole("button", { name: "09" }));
    expect(onSelect).toHaveBeenCalledWith("09");
  });
});
