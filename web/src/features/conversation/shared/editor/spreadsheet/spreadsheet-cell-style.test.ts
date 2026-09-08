// INPUT: Real ExcelJS cells with point-based font sizes and workbook text formatting.
// OUTPUT: CSS preserves source font scale, decoration, wrapping and vertical alignment.
// POS: Spreadsheet projection regression; no browser layout or XLSX decoding is exercised.
import { Workbook } from "exceljs";
import { expect, it } from "vitest";
import { createSpreadsheetCellStyle, getSpreadsheetCellStyle } from "./spreadsheet-cell-style";

it.each([[12, 16], [18, 24], [6, 8]])("converts %s pt text to %s CSS px without a second font clamp", (points, pixels) => {
  const cell = new Workbook().addWorksheet("Sheet").getCell("A1");
  cell.font = { size: points, name: "Test Font", bold: true, italic: true, color: { argb: "FF123456" } };
  const style = getSpreadsheetCellStyle(cell)!;
  expect(style.font?.size).toBe(pixels);
  expect(createSpreadsheetCellStyle([style], 0)).toMatchObject({
    fontSize: pixels, lineHeight: "normal", fontFamily: "Test Font", fontWeight: 700, fontStyle: "italic", color: "#123456",
  });
});

it("keeps default text on one line and applies wrapping and alignment to a flex cell", () => {
  expect(createSpreadsheetCellStyle([])).toMatchObject({ whiteSpace: "nowrap", display: "flex", justifyContent: "flex-start" });
  expect(createSpreadsheetCellStyle([{ textwrap: true, valign: "middle", align: "right", underline: true, strike: true }], 0)).toMatchObject({
    whiteSpace: "pre-wrap", justifyContent: "center", textAlign: "right", textDecoration: "underline line-through",
  });
  expect(createSpreadsheetCellStyle([{ valign: "bottom" }], 0).justifyContent).toBe("flex-end");
});
