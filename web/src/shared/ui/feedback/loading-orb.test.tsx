// INPUT: LoadingOrb 的 active/preparing 语义帧型。
// OUTPUT: 验证固定帧数、CSS 动画挂点与 reduced-motion 可读首帧合同。
// POS: 共享轻量加载指示器回归测试。

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";

describe("LoadingOrb", () => {
  it("selects stable CSS-driven frame variants without injecting styles", () => {
    const { container, rerender } = render(<LoadingOrb />);

    expect(container.querySelector("[data-loading-orb='active']")).toBeTruthy();
    expect(container.querySelectorAll("[data-frame-count='5']")).toHaveLength(5);
    expect(container.querySelectorAll(".ui-loading-orb-frame")).toHaveLength(5);

    rerender(<LoadingOrb variant="preparing" />);
    expect(container.querySelector("[data-loading-orb='preparing']")).toBeTruthy();
    expect(container.querySelectorAll("[data-frame-count='4']")).toHaveLength(4);
    expect(document.head.textContent).not.toContain("_nexus_orb");
  });
});
