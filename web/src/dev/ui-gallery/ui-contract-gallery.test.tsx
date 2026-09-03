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
    expect(screen.getAllByText("Buttons & actions").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Forms & selection").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Resource states").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "Dark" }));

    await waitFor(() => {
      expect(document.documentElement.dataset.theme).toBe("dark");
      expect(window.location.search).toContain("theme=dark");
      expect(window.location.search).toContain("locale=zh");
    });
  });

  it("opens the real shared dialog and closes it through its named action", () => {
    renderGallery();

    fireEvent.click(screen.getByRole("button", { name: "打开标准弹窗" }));
    expect(screen.getByRole("dialog", { name: "共享弹窗契约" })).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog", { name: "共享弹窗契约" })).toBeNull();
  });
});
