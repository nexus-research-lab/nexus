// INPUT: 图标目录值、清除开关、禁用状态与选择命令。
// OUTPUT: 证明图标项复用 Choice pressed 语义，清除复用 Button，并隐藏装饰图片。
// POS: IconPicker DOM 行为测试；Popover 定位与滚动测量由各自所有者覆盖。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import { IconPicker } from "./icon-picker";

describe("IconPicker", () => {
  it("uses shared image-choice and clear-action semantics", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <IconPicker
          iconFamily="agent"
          iconSize="lg"
          maxIcons={2}
          onSelect={onSelect}
          startIconId={4}
          value="5"
        />
      </I18N_CONTEXT.Provider>,
    );

    const idle = screen.getByRole("button", { name: "icon-4" });
    const selected = screen.getByRole("button", { name: "icon-5" });
    expect(idle.getAttribute("aria-pressed")).toBe("false");
    expect(selected.getAttribute("aria-pressed")).toBe("true");
    expect(selected.className).toContain("h-12 w-12");
    expect(selected.className).not.toContain("shadow-");
    expect(selected.querySelector("img")?.getAttribute("alt")).toBe("");

    await user.click(idle);
    expect(onSelect).toHaveBeenCalledWith("4");
    await user.click(screen.getByRole("button", { name: "common.clear" }));
    expect(onSelect).toHaveBeenLastCalledWith("");
  });

  it("keeps every shared choice disabled when the picker is disabled", () => {
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <IconPicker disabled maxIcons={2} onSelect={vi.fn()} value="1" />
      </I18N_CONTEXT.Provider>,
    );

    for (const action of screen.getAllByRole("button")) {
      expect((action as HTMLButtonElement).disabled).toBe(true);
    }
  });
});
