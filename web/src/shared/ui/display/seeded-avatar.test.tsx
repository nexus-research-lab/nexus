// INPUT: 稳定种子与全部共享头像尺寸。
// OUTPUT: 证明数学曲线头像使用确定性图形和语义圆角档位。
// POS: SeededAvatar DOM 合同；颜色与曲线算法由 lib/seeded-avatar 单独负责。

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UiSeededAvatar } from "./seeded-avatar";

const EXPECTED_PRESENTATION = {
  "2xs": ["h-6", "w-6", "radius-control-xs"],
  xs: ["h-8", "w-8", "radius-control-sm"],
  sm: ["h-9", "w-9", "radius-control-sm"],
  md: ["h-10", "w-10", "radius-control-md"],
  lg: ["h-12", "w-12", "radius-control-lg"],
} as const;

describe("UiSeededAvatar", () => {
  it.each(Object.entries(EXPECTED_PRESENTATION))(
    "projects %s through shared size and radius roles",
    (size, expectedClasses) => {
      const { container } = render(
        <UiSeededAvatar seed="stable-resource-id" size={size as keyof typeof EXPECTED_PRESENTATION} />,
      );
      const avatar = container.firstElementChild;

      expect(avatar).not.toBeNull();
      for (const className of expectedClasses) {
        expect(avatar?.className).toContain(className);
      }
      expect(avatar?.className).not.toContain("rounded-[");
      expect(avatar?.querySelector("path")?.getAttribute("d")).toBeTruthy();
    },
  );

  it("keeps the generated curve stable for the same resource identity", () => {
    const first = render(<UiSeededAvatar seed="same-id" />);
    const second = render(<UiSeededAvatar seed="same-id" />);

    expect(first.container.querySelector("path")?.getAttribute("d"))
      .toBe(second.container.querySelector("path")?.getAttribute("d"));
  });
});
