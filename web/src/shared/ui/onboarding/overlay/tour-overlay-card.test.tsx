// INPUT: 带说明、条目和中间步骤导航动作的 Tour step。
// OUTPUT: 证明导览卡使用语义排版并保持跳过、前进和后退按钮行为。
// POS: TourOverlayCard DOM 行为测试；Portal 定位与页面点击退出由 Overlay 测试负责。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { TourOverlayCard } from "./tour-overlay-card";

describe("TourOverlayCard", () => {
  it("uses semantic typography while preserving navigation actions", () => {
    const onClose = vi.fn();
    const onNext = vi.fn();
    const onPrevious = vi.fn();
    render(
      <I18nProvider>
        <TourOverlayCard
          isLastStep={false}
          onClose={onClose}
          onNext={onNext}
          onPrevious={onPrevious}
          placement="bottom"
          step={{
            description: "了解统一组件、交互与页面结构。",
            id: "design-system",
            items: [
              { icon: "puzzle", text: "复用共享组件" },
              { icon: "bot", text: "遵守 Agent 修改规范" },
            ],
            title: "前端设计系统",
          }}
          stepCount={3}
          stepIndex={1}
        />
      </I18nProvider>,
    );

    expect(screen.getByRole("heading", { name: "前端设计系统" }).className)
      .toContain("ui-type-page-title");
    expect(screen.getByText("了解统一组件、交互与页面结构。").className)
      .toContain("ui-type-supporting");
    expect(screen.getByText("复用共享组件").className).toContain("ui-type-metadata");
    expect(screen.getByText("2 / 3").className).toContain("ui-type-caption");

    fireEvent.click(screen.getByRole("button", { name: /back|上一步/i }));
    fireEvent.click(screen.getByRole("button", { name: /next|下一步/i }));
    fireEvent.click(screen.getByRole("button", { name: /skip|跳过/i }));

    expect(onPrevious).toHaveBeenCalledOnce();
    expect(onNext).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledWith({ completed: true });
  });
});
