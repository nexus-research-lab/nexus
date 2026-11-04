// INPUT: Code content, keyboard navigation and clipboard success/failure.
// OUTPUT: Named code groups lead directly to copying exact source and report only success.
// POS: Markdown code interaction including its shell, with rendering and clipboard transport isolated.
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
it.each([{ success: true, language: "javascript" }, { success: false, language: "" }])("copies exact code with keyboard and projects success=$success", async ({ success, language }) => {
  mocks.write.mockResolvedValue(success);
  const error = vi.spyOn(console, "error").mockImplementation(() => {});
  const user = userEvent.setup();
  const value = "const greeting = '你好';\n";
  render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (key, params) => key === "markdown.code.group" ? `${params?.language} 代码` : key }}>
    <button>Before</button>
    <CodeBlockContent language={language} value={value} />
    <button>After</button>
  </I18N_CONTEXT.Provider>);
  expect(screen.getByRole("group", { name: `${language || "text"} 代码` })).toBeTruthy();
  const button = screen.getByRole("button", { name: "markdown.code.copy" });
  screen.getByRole("button", { name: "Before" }).focus();
  await user.tab();
  expect(document.activeElement).toBe(button);
  await user.keyboard("{Enter}");
  expect(mocks.write).toHaveBeenCalledExactlyOnceWith(value);
  expect(button.getAttribute("aria-label")).toBe(success ? "markdown.code.copied" : "markdown.code.copy");
  expect(button.getAttribute("type")).toBe("button");
  await user.tab();
  expect(document.activeElement).toBe(screen.getByRole("button", { name: "After" }));
  error.mockRestore();
});
