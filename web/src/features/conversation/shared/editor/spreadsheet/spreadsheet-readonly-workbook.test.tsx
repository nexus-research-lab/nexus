// INPUT: 包含多个工作表的只读 Workbook、当前索引与切换回调。
// OUTPUT: 证明工作表切换复用全局底线 Tabs，并传递精确索引。
// POS: Spreadsheet 预览视图行为测试；虚拟网格数学由 spreadsheet-grid-model 测试负责。

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import type { SpreadsheetPreviewSheetData } from "./spreadsheet-preview-model";
import { SpreadsheetReadonlyWorkbook } from "./spreadsheet-readonly-workbook";

vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: () => ({ getVirtualItems: () => [] }),
}));

function createSheet(name: string): SpreadsheetPreviewSheetData {
  return {
    column_count: 1,
    columns: {},
    merges: [],
    name,
    row_count: 1,
    rows: {},
    styles: [],
  };
}

describe("SpreadsheetReadonlyWorkbook", () => {
  it("renders shared underline tabs and selects the exact sheet", async () => {
    const user = userEvent.setup();
    const onSelectSheet = vi.fn();

    render(
      <I18nProvider>
        <SpreadsheetReadonlyWorkbook
          activeSheetIndex={0}
          onSelectSheet={onSelectSheet}
          workbook={{ sheets: [createSheet("Overview"), createSheet("Details")] }}
        />
      </I18nProvider>,
    );

    const group = screen.getByRole("group");
    const overview = screen.getByRole("button", { name: "Overview" });
    const details = screen.getByRole("button", { name: "Details" });
    expect(group.className).toContain("overflow-x-auto");
    expect(overview.getAttribute("aria-pressed")).toBe("true");
    expect(overview.className).toContain("border-b-2");
    expect(overview.className).toContain("rounded-none");
    expect(details.getAttribute("title")).toBe("Details");

    await user.click(details);
    expect(onSelectSheet).toHaveBeenCalledWith(1);
  });
});
