// INPUT: Nexus 启动加载态的默认与自定义消息。
// OUTPUT: 证明 live status、语义排版及 reduced-motion 静态品牌帧合同。
// POS: AppLoadingState DOM 行为测试；普通页面加载图标不属于本组件。

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AppLoadingState } from "@/shared/ui/layout/app-loading-screen";

describe("AppLoadingState", () => {
  it("keeps the local brand animation accessible and reduced-motion safe", () => {
    const { container } = render(<AppLoadingState message="正在连接 Nexus" />);

    const status = screen.getByRole("status");
    expect(status.getAttribute("aria-busy")).toBe("true");
    expect(status.getAttribute("aria-live")).toBe("polite");
    expect(screen.getByText("正在连接 Nexus").className).toContain("ui-type-supporting");
    expect(container.querySelector("source")?.getAttribute("srcset"))
      .toBe("/lotties/cat-loading-static.webp");
    expect(container.querySelector("img")?.getAttribute("src"))
      .toBe("/lotties/cat-loading.webp");
  });
});
