// INPUT: Mermaid 首次渲染、已有图表更新状态与原生图形命中区。
// OUTPUT: 证明单一 live status、语义排版、共享 Spinner 与 native button 合同。
// POS: Mermaid view parts DOM 行为测试；不执行 Mermaid 渲染器。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import { MermaidRenderedPreview } from "./mermaid-view-parts";

describe("Mermaid view parts", () => {
  it("keeps rendering status consistent", () => {
    const { container } = render(
      <I18nProvider>
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

    const status = screen.getByRole("status");
    expect(status.getAttribute("aria-busy")).toBe("true");
    expect(status.className).toContain("ui-type-metadata");
    const spinner = container.querySelector('[role="status"] svg');
    expect(spinner?.getAttribute("aria-hidden")).toBe("true");
    expect(spinner?.getAttribute("class")).toContain("motion-reduce:animate-none");
  });

  it("uses one native button for rendered-diagram activation", async () => {
    const user = userEvent.setup();
    const onOpenPreview = vi.fn();
    const { container } = render(
      <I18nProvider>
        <MermaidRenderedPreview
          compact
          constrainHeight
          error={null}
          isRendering={false}
          isStreaming={false}
          onOpenPreview={onOpenPreview}
          svg={'<svg aria-hidden="true" viewBox="0 0 10 10"></svg>'}
        />
      </I18nProvider>,
    );

    const preview = screen.getByRole("button", {
      name: "Enlarge Mermaid chart preview",
    }) as HTMLButtonElement;
    expect(preview.tagName).toBe("BUTTON");
    expect(preview.type).toBe("button");
    expect(container.querySelector('[role="button"]')).toBeNull();

    await user.click(preview);
    preview.focus();
    await user.keyboard("{Enter}");
    expect(onOpenPreview).toHaveBeenCalledTimes(2);
  });
});
