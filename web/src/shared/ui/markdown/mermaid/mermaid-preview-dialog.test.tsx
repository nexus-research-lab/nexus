// INPUT: 放大图表与关闭命令。
// OUTPUT: 画布进入键盘焦点顺序，并保持具名模态与 Escape 退出。
// POS: Mermaid 放大预览可访问性回归，不模拟浏览器滚动几何。
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MermaidPreviewDialog } from "./mermaid-preview-dialog";
afterEach(() => vi.restoreAllMocks());
it("offers a named keyboard scroll region inside the preview", async () => {
  vi.spyOn(HTMLElement.prototype, "getClientRects").mockReturnValue([{} as DOMRect] as unknown as DOMRectList);
  const user = userEvent.setup();
  const close = vi.fn();
  render(<I18N_CONTEXT.Provider value={{ locale: "en", setLocale: vi.fn(), t: (key) => key }}>
    <MermaidPreviewDialog isOpen svg='<svg xmlns="http://www.w3.org/2000/svg" />' onClose={close} />
  </I18N_CONTEXT.Provider>);
  const dialog = screen.getByRole("dialog", { name: "markdown.mermaid.preview_title" });
  const canvas = screen.getByRole("region", { name: "markdown.mermaid.preview_title" });
  expect(dialog.contains(canvas)).toBe(true);
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole("button")));
  await user.tab();
  expect(document.activeElement).toBe(canvas);
  await user.keyboard("{Escape}");
  expect(close).toHaveBeenCalledOnce();
});
