// INPUT: Mermaid 模式动作、首次渲染和已有图表更新状态。
// OUTPUT: 证明 tab 交互、单一 live status、语义排版与共享 Spinner 合同。
// POS: Mermaid view parts DOM 行为测试；不执行 Mermaid 渲染器。

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { MermaidModeButton, MermaidRenderedPreview } from "./mermaid-view-parts";

describe("Mermaid view parts", () => {
  it("keeps mode actions interactive and rendering status consistent", () => {
    const onModeClick = vi.fn();
    const { container } = render(
      <I18nProvider>
        <MermaidModeButton active onClick={onModeClick}>源码</MermaidModeButton>
        <MermaidRenderedPreview
          compact
          constrainHeight
          error={null}
          isRendering
          isStreaming={false}
          onOpenPreview={vi.fn()}
          svg=""
        />
      </I18nProvider>,
    );

    fireEvent.click(screen.getByRole("tab", { name: "源码" }));
    expect(onModeClick).toHaveBeenCalledOnce();

    const status = screen.getByRole("status");
    expect(status.getAttribute("aria-busy")).toBe("true");
    expect(status.className).toContain("ui-type-metadata");
    const spinner = container.querySelector('[role="status"] svg');
    expect(spinner?.getAttribute("aria-hidden")).toBe("true");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });
});
