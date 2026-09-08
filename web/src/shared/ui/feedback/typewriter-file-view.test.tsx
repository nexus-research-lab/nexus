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

it("changes the line-count language without replacing or altering the source", () => {
  const { container, rerender } = render(source("first\nsecond"));
  const pre = container.querySelector("pre");
  expect(screen.getByText("2 lines")).toBeTruthy();
  rerender(source("first\nsecond", "zh"));
  expect(screen.getByText("2 行")).toBeTruthy();
  expect(container.querySelector("pre")).toBe(pre);
  expect(pre?.textContent).toBe("first\nsecond");
});

it("keeps following appended file content without injecting runtime styles", () => {
  const styles = document.head.querySelectorAll("style").length;
  const { container, rerender } = render(source("first"));
  const pre = container.querySelector("pre")!;
  Object.defineProperty(pre, "scrollHeight", { configurable: true, value: 420 });
  rerender(source("first\nsecond"));
  expect(pre.scrollTop).toBe(420);
  expect(document.head.querySelectorAll("style")).toHaveLength(styles);
});
