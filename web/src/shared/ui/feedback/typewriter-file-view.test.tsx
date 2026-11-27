// INPUT: Growing source content and locale changes.
// OUTPUT: Exact source text, logical line counts and retained bottom-follow behavior.
// POS: Offline streaming source regressions; no viewport or visual verification.

import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT, type I18nContextValue } from "@/shared/i18n/i18n-context";
import { MESSAGES, type Locale } from "@/shared/i18n/messages";
import { TypewriterFileView } from "./typewriter-file-view";

function source(content: string, locale: Locale = "en") {
  const t: I18nContextValue["t"] = (key, params) => MESSAGES[locale][key].replace("{count}", String(params?.count ?? ""));
  return <I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t }}>
    <TypewriterFileView content={content} />
  </I18N_CONTEXT.Provider>;
}

it("counts file lines before any container measurement and preserves all text", () => {
  const { container, rerender } = render(source(""));
  expect(screen.getByText("1 line")).toBeTruthy();
  const oneLongLine = "超长源码👩🏽‍💻 ".repeat(300);
  rerender(source(oneLongLine));
  expect(screen.getByText("1 line")).toBeTruthy();
  expect(container.querySelector("pre")?.textContent).toBe(oneLongLine);
  const contents = "const value = 1;\r\n中文\rnext\n";
  rerender(source(contents));
  expect(screen.getByText("4 lines")).toBeTruthy();
  expect(container.querySelector("pre")?.textContent).toBe(contents);
});
