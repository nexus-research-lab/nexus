// INPUT: 已解析的多页演示文稿、当前语言与翻页/缩略图选择动作。
// OUTPUT: 证明预览使用共享 Choice/IconButton，并保持页码、选中态和边界禁用行为。
// POS: 演示文稿预览视图行为测试；PPTX 解包和 Canvas 绘制由各自测试负责。

import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";

import type { PresentationSlide } from "./presentation-preview-model";
import { PresentationFilePreview } from "./presentation-file-preview";

vi.mock("../office-preview-resource", () => ({
  fetchOfficePreviewBuffer: vi.fn(async () => new ArrayBuffer(8)),
}));

vi.mock("./presentation-pptx-parser", () => ({
  parsePptx: vi.fn(async () => ({
    objectUrls: [],
    slides: [
      {
        background: "#fff",
        elements: [],
        height: 720,
        id: "slide-1",
        title: "Overview",
        width: 1280,
      },
      {
        background: "#fff",
        elements: [],
        height: 720,
        id: "slide-2",
        title: "Details",
        width: 1280,
      },
    ],
  })),
}));

vi.mock("./presentation-slide-canvas", () => ({
  PresentationSlideCanvas: ({
    slide,
    thumbnail,
  }: {
    slide: PresentationSlide;
    thumbnail?: boolean;
  }) => (
    <div data-thumbnail={thumbnail ? "true" : "false"}>
      {thumbnail ? null : slide.title}
    </div>
  ),
}));

describe("PresentationFilePreview", () => {
  it("shares selection and paging controls without losing boundary behavior", async () => {
    const user = userEvent.setup();
    render(
      <I18N_CONTEXT.Provider
        value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}
      >
        <PresentationFilePreview
          agentId="agent-1"
          fileName="deck.pptx"
          isPreviewFocused={false}
          onTogglePreviewFocus={vi.fn()}
          path="slides/deck.pptx"
        />
      </I18N_CONTEXT.Provider>,
    );

    const next = await screen.findByRole("button", {
      name: "workspace_file.next_slide",
    });
    const previous = screen.getByRole("button", {
      name: "workspace_file.previous_slide",
    });
    const firstThumbnail = screen.getByText("1. Overview").closest("button");
    const secondThumbnail = screen.getByText("2. Details").closest("button");

    expect(firstThumbnail?.getAttribute("aria-pressed")).toBe("true");
    expect(secondThumbnail?.getAttribute("aria-pressed")).toBe("false");
    expect((previous as HTMLButtonElement).disabled).toBe(true);
    expect((next as HTMLButtonElement).disabled).toBe(false);
    expect(next.className).toContain("h-8");
    expect(next.className).toContain("radius-control-md");

    await user.click(next);

    await waitFor(() => {
      expect(screen.getByText("2 / 2 · Details")).toBeTruthy();
      expect((next as HTMLButtonElement).disabled).toBe(true);
      expect((previous as HTMLButtonElement).disabled).toBe(false);
      expect(secondThumbnail?.getAttribute("aria-pressed")).toBe("true");
    });
  });
});
