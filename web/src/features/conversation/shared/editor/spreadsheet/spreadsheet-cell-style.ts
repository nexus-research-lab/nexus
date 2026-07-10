import type {
  Alignment,
  Border,
  Cell,
  Color,
  Fill,
  Font,
} from "exceljs";

const EXCEL_ROW_HEIGHT_TO_PX = 4 / 3;
const THEME_COLORS = [
  "#ffffff", "#000000", "#bfbfbf", "#323232", "#4472c4",
  "#ed7d31", "#a5a5a5", "#ffc000", "#5b9bd5", "#71ad47",
];
const INDEXED_COLORS = [
  "#000000", "#ffffff", "#ff0000", "#00ff00", "#0000ff", "#ffff00",
  "#ff00ff", "#00ffff", "#000000", "#ffffff", "#ff0000", "#00ff00",
  "#0000ff", "#ffff00", "#ff00ff", "#00ffff", "#800000", "#008000",
  "#000080", "#808000", "#800080", "#008080", "#c0c0c0", "#808080",
  "#9999ff", "#993366", "#ffffcc", "#ccffff", "#660066", "#ff8080",
  "#0066cc", "#ccccff", "#000080", "#ff00ff", "#ffff00", "#00ffff",
  "#800080", "#800000", "#008080", "#0000ff", "#00ccff", "#ccffff",
  "#ccffcc", "#ffff99", "#99ccff", "#ff99cc", "#cc99ff", "#ffcc99",
  "#3366ff", "#33cccc", "#99cc00", "#ffcc00", "#ff9900", "#ff6600",
  "#666699", "#969696", "#003366", "#339966", "#003300", "#333300",
  "#993300", "#993366", "#333399", "#333333", "#000000",
];

export type SpreadsheetPreviewBorderSide = [string, string];

export interface SpreadsheetPreviewCellStyle {
  align?: "left" | "center" | "right";
  bgcolor?: string;
  border?: Partial<Record<
    "top" | "right" | "bottom" | "left",
    SpreadsheetPreviewBorderSide
  >>;
  color?: string;
  font?: {
    bold?: boolean;
    italic?: boolean;
    name?: string;
    size?: number;
  };
  strike?: boolean;
  textwrap?: boolean;
  underline?: boolean;
  valign?: "top" | "middle" | "bottom";
}

const HORIZONTAL_ALIGNMENT: Partial<Record<
  NonNullable<Alignment["horizontal"]>,
  SpreadsheetPreviewCellStyle["align"]
>> = {
  center: "center",
  centerContinuous: "center",
  distributed: "left",
  fill: "left",
  justify: "left",
  left: "left",
  right: "right",
};
const VERTICAL_ALIGNMENT: Partial<Record<
  NonNullable<Alignment["vertical"]>,
  SpreadsheetPreviewCellStyle["valign"]
>> = {
  bottom: "bottom",
  distributed: "top",
  justify: "top",
  middle: "middle",
  top: "top",
};

/** ExcelJS 样式在此转换，预览模型不依赖运行时渲染字段细节。 */
export function getSpreadsheetCellStyle(
  cell: Cell,
): SpreadsheetPreviewCellStyle | undefined {
  const font = getFontStyle(cell.font);
  const style: SpreadsheetPreviewCellStyle = {
    align: cell.alignment?.horizontal
      ? HORIZONTAL_ALIGNMENT[cell.alignment.horizontal]
      : undefined,
    bgcolor: getFillColor(cell.fill),
    border: getBorderStyle(cell.border),
    color: getExcelColor(cell.font?.color),
    font,
    strike: cell.font?.strike || undefined,
    textwrap: cell.alignment?.wrapText || undefined,
    underline: Boolean(
      cell.font?.underline && cell.font.underline !== "none",
    ) || undefined,
    valign: cell.alignment?.vertical
      ? VERTICAL_ALIGNMENT[cell.alignment.vertical]
      : undefined,
  };
  return Object.values(style).some((value) => value !== undefined)
    ? style
    : undefined;
}

function getFontStyle(
  font?: Partial<Font>,
): SpreadsheetPreviewCellStyle["font"] | undefined {
  if (!font) {
    return undefined;
  }
  const result: NonNullable<SpreadsheetPreviewCellStyle["font"]> = {
    bold: font.bold || undefined,
    italic: font.italic || undefined,
    name: font.name || undefined,
    size: font.size
      ? Math.round(font.size / EXCEL_ROW_HEIGHT_TO_PX)
      : undefined,
  };
  return Object.values(result).some((value) => value !== undefined)
    ? result
    : undefined;
}

function getFillColor(fill?: Fill): string | undefined {
  if (!fill || fill.type !== "pattern") {
    return undefined;
  }
  return getExcelColor(fill.fgColor) ?? getExcelColor(fill.bgColor);
}

function getBorderStyle(
  border?: Cell["border"],
): SpreadsheetPreviewCellStyle["border"] | undefined {
  if (!border) {
    return undefined;
  }
  const result: NonNullable<SpreadsheetPreviewCellStyle["border"]> = {
    bottom: getBorderSide(border.bottom),
    left: getBorderSide(border.left),
    right: getBorderSide(border.right),
    top: getBorderSide(border.top),
  };
  return Object.values(result).some(Boolean) ? result : undefined;
}

function getBorderSide(
  border?: Partial<Border>,
): SpreadsheetPreviewBorderSide | undefined {
  if (!border?.style) {
    return undefined;
  }
  return [border.style, getExcelColor(border.color) ?? "#d1d5db"];
}

function getExcelColor(
  color?: Partial<Color> | null,
): string | undefined {
  const runtimeColor = color as
    | (Partial<Color> & { indexed?: number })
    | null
    | undefined;
  if (!runtimeColor) {
    return undefined;
  }
  if (runtimeColor.argb) {
    const hex = runtimeColor.argb.replace(/^#/, "");
    const normalized = {
      6: `#${hex}`,
      8: `#${hex.slice(2)}`,
    }[hex.length];
    if (normalized && /^[a-f\d]+$/i.test(hex)) {
      return normalized;
    }
  }
  if (typeof runtimeColor.theme === "number") {
    return THEME_COLORS[runtimeColor.theme];
  }
  if (typeof runtimeColor.indexed === "number") {
    return INDEXED_COLORS[runtimeColor.indexed];
  }
  return undefined;
}
