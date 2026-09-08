// INPUT: Spinner 的有限尺寸、颜色和调用方外部布局 class。
// OUTPUT: 证明所有旋转图标共享尺寸阶梯、语义颜色与 reduced-motion 合同。
// POS: Spinner 视觉映射单元测试；具体图标和可访问状态由消费者负责。

import { describe, expect, it } from "vitest";

import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

describe("getUiSpinnerClassName", () => {
  it("combines one size and tone with the shared motion contract", () => {
    const className = getUiSpinnerClassName(
      { size: "xl", tone: "primary" },
      "mx-auto",
    );

    expect(className).toContain("h-6 w-6");
    expect(className).toContain("text-primary");
    expect(className).toContain("animate-spin");
    expect(className).toContain("motion-reduce:animate-none");
    expect(className).toContain("mx-auto");
  });

  it("keeps the large preview loader on the same semantic size scale", () => {
    expect(getUiSpinnerClassName({ size: "2xl" })).toContain("h-8 w-8");
  });
});
