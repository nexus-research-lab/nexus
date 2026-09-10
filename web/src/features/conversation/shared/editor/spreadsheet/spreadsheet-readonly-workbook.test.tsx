// INPUT: 包含多个工作表的只读 Workbook、当前索引与切换回调。
// OUTPUT: 共享 Tabs、原生滚动入口和虚拟行/合并单元格的精确语义。
// POS: Spreadsheet 离线 DOM 回归；虚拟视窗是受控输入，真实投影与格式函数参与渲染。

import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "@/shared/i18n/i18n-provider";

import type { SpreadsheetPreviewSheetData } from "./spreadsheet-preview-model";
import { SpreadsheetReadonlyWorkbook } from "./spreadsheet-readonly-workbook";

const viewport = vi.hoisted(() => ({ rows: [0, 1], columns: [0, 1] }));
vi.mock("@tanstack/react-virtual", () => ({
  useVirtualizer: (options: { count: number; horizontal?: boolean; estimateSize: (index: number) => number }) => ({
    getVirtualItems: () => (options.horizontal ? viewport.columns : viewport.rows).filter((index) => index < options.count).map((index) => {
      const start = Array.from({ length: index }, (_, i) => options.estimateSize(i)).reduce((sum, size) => sum + size, 0);
      const size = options.estimateSize(index);
      return { key: index, index, size, start, end: start + size, lane: 0 };
    }),
  }),
}));
beforeEach(() => { viewport.rows = [0, 1]; viewport.columns = [0, 1]; });

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
  it("exposes ordered virtual rows and offscreen merge anchors as a named read-only table", async () => {
    viewport.rows = [2, 3]; viewport.columns = [2, 3];
    const sheet: SpreadsheetPreviewSheetData = {
      ...createSheet("Merged report"), row_count: 5, column_count: 4,
      merges: [{ ref: "A1:C3", start_row: 0, end_row: 2, start_col: 0, end_col: 2 }],
      rows: { 0: { cells: { 0: { text: "Merged title", style: 0 } } }, 3: { cells: { 3: { text: "Last" } } } },
      styles: [{ font: { size: 16 }, valign: "middle" }],
    };
    const view = render(<I18nProvider><SpreadsheetReadonlyWorkbook activeSheetIndex={0} onSelectSheet={vi.fn()} workbook={{ sheets: [sheet] }} /></I18nProvider>);
    const table = screen.getByRole("table", { name: sheet.name });
    expect(screen.queryByRole("grid")).toBeNull();
    expect(table.getAttribute("aria-rowcount")).toBe("5");
    expect(table.getAttribute("aria-colcount")).toBe("4");
    const rows = within(table).getAllByRole("row");
    expect(rows.map((row) => row.getAttribute("aria-rowindex"))).toEqual(["1", "3", "4"]);
    expect(rows.map((row) => within(row).getAllByRole("cell").map((cell) => cell.getAttribute("aria-colindex")))).toEqual([["1"], ["4"], ["3", "4"]]);
    const merge = within(table).getByRole("cell", { name: "Merged title" });
    expect(merge.getAttribute("aria-rowspan")).toBe("3");
    expect(merge.getAttribute("aria-colspan")).toBe("3");
    expect(merge.style.height).toBe("78px");
    expect(merge.style.width).toBe("288px");
    expect(merge.style.fontSize).toBe("16px");
    expect(merge.style.justifyContent).toBe("center");

    const region = screen.getByRole("region", { name: sheet.name });
    await userEvent.setup().tab();
    expect(document.activeElement).toBe(region);
    region.scrollLeft = 96; region.scrollTop = 52;
    fireEvent.scroll(region);
    const headerTransforms = [...view.container.querySelectorAll('[aria-hidden="true"] > div')].map((element) => (element as HTMLElement).style.transform);
    expect(headerTransforms).toEqual(["translateX(-96px)", "translateY(-52px)"]);
  });

  it("resets the scroll viewport when selecting a different sheet", () => {
    const workbook = { sheets: [createSheet("First"), createSheet("Second")] };
    const renderWorkbook = (index: number) => <I18nProvider><SpreadsheetReadonlyWorkbook activeSheetIndex={index} onSelectSheet={vi.fn()} workbook={workbook} /></I18nProvider>;
    const view = render(renderWorkbook(0));
    const first = screen.getByRole("region", { name: "First" });
    first.scrollTop = 200; first.scrollLeft = 100; fireEvent.scroll(first);
    view.rerender(renderWorkbook(1));
    const second = screen.getByRole("region", { name: "Second" });
    expect(second).not.toBe(first);
    expect(second.scrollTop).toBe(0); expect(second.scrollLeft).toBe(0);
  });

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
    expect(details.getAttribute("title")).toBeNull();

    await user.click(details);
    expect(onSelectSheet).toHaveBeenCalledWith(1);
  });
});
