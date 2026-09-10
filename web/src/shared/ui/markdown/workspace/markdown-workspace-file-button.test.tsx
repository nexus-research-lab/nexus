// INPUT: 本地化文件标签和已解析的工作区路径。
// OUTPUT: 键盘打开保留精确路径，提示按当前语言显示。
// POS: Markdown 内联文件按钮回归，不请求文件接口。
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, it, vi } from "vitest";
import { I18N_CONTEXT } from "@/shared/i18n/i18n-context";
import { MESSAGES } from "@/shared/i18n/messages";
import { WorkspaceFileButton } from "./markdown-workspace-file-button";
it.each(["zh", "en"] as const)("localizes the path hint and opens the exact file in %s", async (locale) => {
  const user = userEvent.setup();
  const open = vi.fn();
  const path = "reports/结果 v2.md";
  render(<I18N_CONTEXT.Provider value={{ locale, setLocale: vi.fn(), t: (key) => MESSAGES[locale][key].replace("{path}", path) }}>
    <WorkspaceFileButton label="报告" path={path} onOpenWorkspaceFile={open} />
  </I18N_CONTEXT.Provider>);
  const button = screen.getByRole("button", { name: "报告" });
  await user.hover(button);
  expect((await screen.findByRole("tooltip")).textContent).toBe(`${locale === "zh" ? "打开" : "Open"} ${path}`);
  await user.tab();
  await user.keyboard("{Enter}");
  expect(open).toHaveBeenCalledExactlyOnceWith(path);
});
