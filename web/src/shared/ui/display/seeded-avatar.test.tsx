// INPUT: 同一资源身份的稳定种子。
// OUTPUT: 证明数学曲线头像使用确定性图形。
// POS: SeededAvatar DOM 合同；颜色与曲线算法由 lib/seeded-avatar 单独负责。

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { UiSeededAvatar } from "./seeded-avatar";

describe("UiSeededAvatar", () => {
  it("keeps the generated curve stable for the same resource identity", () => {
    const first = render(<UiSeededAvatar seed="same-id" />);
    const second = render(<UiSeededAvatar seed="same-id" />);

    expect(first.container.querySelector("path")?.getAttribute("d"))
      .toBe(second.container.querySelector("path")?.getAttribute("d"));
  });
});
