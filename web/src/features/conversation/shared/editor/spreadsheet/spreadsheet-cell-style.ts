import type {
  Alignment,
  Border,
  Cell,
  Color,
  Fill,
  Font,
} from "exceljs";
import type { CSSProperties } from "react";

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

type SpreadsheetPreviewBorderSide = [string, string];

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

const BORDER_LINE_STYLE: Record<string, string> = {
  dashed: "dashed",
  dotted: "dotted",
  double: "double",
};
const WIDE_BORDER_KINDS = new Set(["medium", "thick"]);

export function createSpreadsheetCellStyle(
  styles: readonly SpreadsheetPreviewCellStyle[],
  styleIndex?: number,
): CSSProperties {
  const previewStyle = resolvePreviewStyle(styles, styleIndex);
  if (!previewStyle) {
    return {};
  }

  return {
    backgroundColor: previewStyle.bgcolor,
    ...createBorderStyles(previewStyle.border),
    color: previewStyle.color,
    ...createFontCss(previewStyle),
    textAlign: previewStyle.align,
    textDecoration: createTextDecoration(previewStyle),
    verticalAlign: previewStyle.valign,
    whiteSpace: previewStyle.textwrap ? "pre-wrap" : "nowrap",
  };
}

function resolvePreviewStyle(
  styles: readonly SpreadsheetPreviewCellStyle[],
  styleIndex?: number,
): SpreadsheetPreviewCellStyle | undefined {
  return styleIndex === undefined ? undefined : styles[styleIndex];
}

function createBorderStyles(
  border?: SpreadsheetPreviewCellStyle["border"],
): CSSProperties {
  return {
    borderTop: createBorderCss(border?.top),
    borderRight: createBorderCss(border?.right),
    borderBottom: createBorderCss(border?.bottom),
    borderLeft: createBorderCss(border?.left),
  };
}

function createFontCss(
  previewStyle: SpreadsheetPreviewCellStyle,
): CSSProperties {
  const font = previewStyle.font;
  if (!font) {
    return {};
  }
  return {
    fontFamily: font.name,
    fontSize: font.size ? Math.max(10, font.size) : undefined,
    fontStyle: font.italic ? "italic" : undefined,
    fontWeight: font.bold ? 700 : undefined,
  };
}

function createTextDecoration(
  previewStyle: SpreadsheetPreviewCellStyle,
): string | undefined {
  return [
    previewStyle.underline ? "underline" : "",
    previewStyle.strike ? "line-through" : "",
  ].filter(Boolean).join(" ") || undefined;
}

function createBorderCss(
  border?: SpreadsheetPreviewBorderSide,
): string | undefined {
  if (!border) {
    return undefined;
  }
  const [kind, color] = border;
  const lineStyle = BORDER_LINE_STYLE[kind] ?? "solid";
  const width = WIDE_BORDER_KINDS.has(kind) ? 2 : 1;
  return `${width}px ${lineStyle} ${color}`;
}

/** ExcelJS 样式在此转换，预览模型不依赖运行时渲染字段细节。 */
export function getSpreadsheetCellStyle(
  cell: Cell,
): SpreadsheetPreviewCellStyle | undefined {
  const font = getFontStyle(cell.font);
  const style: SpreadsheetPreviewCellStyle = {
    align: getHorizontalAlignment(cell.alignment),
    bgcolor: getFillColor(cell.fill),
    border: getBorderStyle(cell.border),
    color: getExcelColor(cell.font?.color),
    font,
    strike: enabledStyle(cell.font?.strike),
    textwrap: enabledStyle(cell.alignment?.wrapText),
    underline: getUnderlineStyle(cell.font),
    valign: getVerticalAlignment(cell.alignment),
  };
  return Object.values(style).some((value) => value !== undefined)
    ? style
    : undefined;
}

function getHorizontalAlignment(
  alignment?: Partial<Alignment>,
): SpreadsheetPreviewCellStyle["align"] {
  const value = alignment?.horizontal;
  return value ? HORIZONTAL_ALIGNMENT[value] : undefined;
}

function getVerticalAlignment(
  alignment?: Partial<Alignment>,
): SpreadsheetPreviewCellStyle["valign"] {
  const value = alignment?.vertical;
  return value ? VERTICAL_ALIGNMENT[value] : undefined;
}

function enabledStyle(value?: boolean): true | undefined {
  return value ? true : undefined;
}

function getUnderlineStyle(font?: Partial<Font>): true | undefined {
  const underline = font?.underline;
  if (!underline || underline === "none") {
    return undefined;
  }
  return true;
}

function getFontStyle(
  font?: Partial<Font>,
): SpreadsheetPreviewCellStyle["font"] | undefined {
  if (!font) {
    return undefined;
  }
  const result: NonNullable<SpreadsheetPreviewCellStyle["font"]> = {
    bold: enabledStyle(font.bold),
    italic: enabledStyle(font.italic),
    name: font.name ?? undefined,
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
