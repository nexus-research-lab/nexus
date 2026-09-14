// INPUT: Code content and clipboard success/failure.
// OUTPUT: Copy uses exact source text and reports only successful completion.
// POS: Markdown copy action behavior, with rendering and clipboard transport isolated.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { CodeBlockContent } from "./code-block-content";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
const mocks = vi.hoisted(() => ({ write: vi.fn() }));
vi.mock("@/shared/lib/browser/clipboard", () => ({ writeTextToClipboard: mocks.write }));
vi.mock("@/shared/theme/theme-context", () => ({ useTheme: () => ({ theme: "light" }) }));
vi.mock("react-syntax-highlighter", () => ({ PrismAsyncLight: ({ children }: { children: string }) => <pre>{children}</pre> }));
beforeEach(() => { mocks.write.mockReset(); });
it.each([true, false])("copies exact code and projects success=%s", async (success) => {
  mocks.write.mockResolvedValue(success);
  const error = vi.spyOn(console, "error").mockImplementation(() => {});
  const user = userEvent.setup();
  const value = "const greeting = '你好';\n";
  render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key) => key }}>
    <CodeBlockContent language="javascript" value={value} />
  </I18N_CONTEXT.Provider>);
  const button = screen.getByRole("button", { name: "markdown.code.copy" });
  await user.click(button);
  expect(mocks.write).toHaveBeenCalledExactlyOnceWith(value);
  expect(button.getAttribute("aria-label")).toBe(success ? "markdown.code.copied" : "markdown.code.copy");
  expect(button.getAttribute("type")).toBe("button");
  error.mockRestore();
});
