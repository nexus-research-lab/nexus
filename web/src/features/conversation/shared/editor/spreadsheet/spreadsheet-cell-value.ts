import type {
  Cell,
  CellErrorValue,
  CellFormulaValue,
  CellHyperlinkValue,
  CellRichTextValue,
  CellSharedFormulaValue,
  CellValue,
} from "exceljs";

interface CellValueFormatRule {
  format: (value: CellValue) => string;
  matches: (value: CellValue) => boolean;
}

function hasProperty(value: CellValue, property: string): boolean {
  return typeof value === "object" && value !== null && property in value;
}

function isPrimitiveValue(value: CellValue): boolean {
  return typeof value === "number"
    || typeof value === "boolean"
    || typeof value === "string";
}

function isFormulaValue(value: CellValue): boolean {
  return hasProperty(value, "formula") || hasProperty(value, "sharedFormula");
}

function formatDateValue(value: CellValue): string {
  const date = value as Date;
  return Number.isNaN(date.getTime()) ? "" : date.toLocaleString();
}

function formatFormulaValue(value: CellValue): string {
  const formula = value as CellFormulaValue | CellSharedFormulaValue;
  return formatSpreadsheetCellValue(formula.result);
}

function formatRichTextValue(value: CellValue): string {
  return (value as CellRichTextValue).richText
    .map((part) => part.text)
    .join("");
}

function formatHyperlinkValue(value: CellValue): string {
  return (value as CellHyperlinkValue).text;
}

function formatErrorValue(value: CellValue): string {
  return (value as CellErrorValue).error;
}

const EMPTY_VALUE_RULE: CellValueFormatRule = {
  format: () => "",
  matches: () => true,
};

const CELL_VALUE_FORMAT_RULES: CellValueFormatRule[] = [
  {
    format: () => "",
    matches: (value) => value === null || value === undefined,
  },
  {
    format: formatDateValue,
    matches: (value) => value instanceof Date,
  },
  {
    format: (value) => String(value),
    matches: isPrimitiveValue,
  },
  {
    format: formatFormulaValue,
    matches: isFormulaValue,
  },
  {
    format: formatRichTextValue,
    matches: (value) => hasProperty(value, "richText"),
  },
  {
    format: formatHyperlinkValue,
    matches: (value) => hasProperty(value, "hyperlink"),
  },
  {
    format: formatErrorValue,
    matches: (value) => hasProperty(value, "error"),
  },
];

function formatSpreadsheetCellValue(value: CellValue): string {
  // ExcelJS 的 CellValue 是封闭联合；默认规则只承接未来新增成员，避免视图静默猜测对象结构。
  const rule = CELL_VALUE_FORMAT_RULES.find((candidate) => candidate.matches(value))
    ?? EMPTY_VALUE_RULE;
  return rule.format(value);
}

export function getSpreadsheetCellText(cell: Cell): string {
  return formatSpreadsheetCellValue(cell.value) || cell.text || "";
}
