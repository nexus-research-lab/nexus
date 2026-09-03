import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { UiContractGallery } from "@/dev/ui-gallery/ui-contract-gallery";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import { THEME_STORAGE_KEY } from "@/shared/theme/theme-context";
import { ThemeProvider } from "@/shared/theme/theme-provider";

function renderGallery() {
  return render(
    <ThemeProvider>
      <I18nProvider>
        <UiContractGallery />
      </I18nProvider>
    </ThemeProvider>,
  );
}

describe("UI contract gallery", () => {
  beforeEach(() => {
    window.localStorage.setItem(THEME_STORAGE_KEY, "light");
    window.localStorage.setItem(LOCALE_STORAGE_KEY, "zh");
    window.history.replaceState(null, "", "/ui-gallery.html");
  });

  it("renders the shared foundation inventory and keeps theme links reproducible", async () => {
    renderGallery();

    expect(screen.getByRole("heading", { name: "Nexus UI Contract Gallery" })).toBeTruthy();
    expect(screen.getAllByText("按钮与动作").length).toBeGreaterThan(0);
    expect(screen.getAllByText("字体层级").length).toBeGreaterThan(0);
    expect(screen.getAllByText("表单与选择").length).toBeGreaterThan(0);
    expect(screen.getAllByText("资源状态").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe("dark");
      expect(window.location.search).toContain("theme=dark");
      expect(window.location.search).toContain("locale=zh");
      expect(window.location.search).toContain("section=foundation");
    });
  });

  it("renders the shared semantic typography roles", () => {
    const { container } = renderGallery();

    expect(container.querySelector('[data-typography-role="display"] .ui-type-display')).toBeTruthy();
    expect(container.querySelector('[data-typography-role="pageTitle"] .ui-type-page-title')).toBeTruthy();
    expect(container.querySelector('[data-typography-role="body"] .ui-type-body')).toBeTruthy();
    expect(container.querySelector('[data-typography-role="code"] .ui-type-code')).toBeTruthy();
  });

  it("opens the real shared dialog and closes it through its named action", () => {
    renderGallery();

    fireEvent.click(screen.getByRole("button", { name: "打开标准弹窗" }));
    expect(screen.getByRole("dialog", { name: "共享弹窗契约" })).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog", { name: "共享弹窗契约" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "打开紧凑弹窗" }));
    const compactDialog = screen.getByRole("dialog", { name: "紧凑弹窗契约" });
    expect(compactDialog.querySelector(".ui-dialog-viewport-compact")).toBeTruthy();
  });

  it("renders the native select primitive as a controlled accessible field", () => {
    renderGallery();

    const select = screen.getByRole("combobox", { name: "原生角色" }) as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "admin" } });
    expect(select.value).toBe("admin");
  });

  it("exposes the shared compact prompt used by workspace create and rename", () => {
    renderGallery();

    fireEvent.click(screen.getByRole("button", { name: "新建文件夹弹窗" }));
    const prompt = screen.getByRole("dialog", { name: "新建文件夹" });
    expect(prompt.querySelector(".max-w-sm")).toBeTruthy();
    expect(screen.getByRole("textbox", { name: "例如：新文件夹" })).toBeTruthy();
  });

  it("exposes both attachment viewer viewport contracts", () => {
    renderGallery();

    fireEvent.click(screen.getByRole("button", { name: "视觉预览尺寸" }));
    const visual = screen.getByRole("dialog", { name: "视觉预览契约" });
    expect(visual.querySelector(".ui-dialog-viewport-visual-preview")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));

    fireEvent.click(screen.getByRole("button", { name: "文档预览尺寸" }));
    const document = screen.getByRole("dialog", { name: "文档预览契约" });
    expect(document.querySelector(".ui-dialog-viewport-document-preview")).toBeTruthy();
  });

  it("switches the fixture copy and document language instead of only changing a selector", async () => {
    renderGallery();

    fireEvent.click(screen.getByRole("button", { name: "English" }));

    await waitFor(() => {
      expect(screen.getByText("Foundation completeness")).toBeTruthy();
      expect(screen.getByRole("button", { name: "New conversation" })).toBeTruthy();
      expect(document.documentElement.lang).toBe("en");
      expect(window.location.search).toContain("locale=en");
    });
  });

  it("uses the section control to render the complete coverage index", async () => {
    renderGallery();

    fireEvent.click(screen.getByRole("button", { name: "覆盖清单" }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "完整覆盖清单" })).toBeTruthy();
      expect(screen.getAllByText("UiButton").length).toBeGreaterThan(0);
      expect(screen.queryByRole("heading", { name: "字体层级" })).toBeNull();
      expect(window.location.search).toContain("section=coverage");
    });
  });
});
