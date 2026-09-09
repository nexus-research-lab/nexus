// INPUT: Blocked preference storage and browser language.
// OUTPUT: Usable localized content and document language without persistence.
// POS: I18n Provider startup and language-change regression.
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18nProvider } from "./i18n-provider";
import { useI18n } from "./i18n-context";
function Content() {
  const { locale, setLocale, t } = useI18n();
  return <button onClick={() => setLocale("zh")}>{locale}:{t("state.retry")}</button>;
}
it("keeps startup and language switching usable when storage is denied", () => {
  const language = vi.spyOn(window.navigator, "language", "get").mockReturnValue("en-US");
  const read = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new Error("blocked read"); });
  const write = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked write"); });
  try {
    render(<I18nProvider><Content /></I18nProvider>);
    expect(document.documentElement.lang).toBe("en");
    fireEvent.click(screen.getByRole("button", {name: "en:Retry"}));
    expect(screen.getByRole("button", {name: "zh:重试"})).toBeTruthy();
    expect(document.documentElement.lang).toBe("zh-CN");
  } finally { read.mockRestore(); write.mockRestore(); language.mockRestore(); }
});
