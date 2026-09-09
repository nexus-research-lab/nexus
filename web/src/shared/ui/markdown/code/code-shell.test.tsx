// INPUT: Code group language and an optional copy action.
// OUTPUT: Keyboard skips decorative shell and reaches its actual action directly.
// POS: Shared code shell accessibility regression.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { CodeShell } from "./code-shell";
it("names the code group and avoids an extra tab stop", async () => {
 const user = userEvent.setup();
 render(<I18N_CONTEXT.Provider value={{ locale: "zh", setLocale: vi.fn(), t: (_key, args) => `${args?.language} 代码` }}>
  <button>之前</button>
  <CodeShell language="javascript" rightSlot={<button>复制</button>}><pre>example</pre></CodeShell>
  <CodeShell><pre>streaming</pre></CodeShell>
  <button>之后</button>
 </I18N_CONTEXT.Provider>);
 expect(screen.getByRole("group", { name: "javascript 代码" })).toBeTruthy();
 expect(screen.getByRole("group", { name: "text 代码" })).toBeTruthy();
 screen.getByRole("button", { name: "之前" }).focus();
 await user.tab(); expect(document.activeElement).toBe(screen.getByRole("button", { name: "复制" }));
 await user.tab(); expect(document.activeElement).toBe(screen.getByRole("button", { name: "之后" }));
});
