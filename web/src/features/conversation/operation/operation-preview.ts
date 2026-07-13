import type { NexusOperationEvent } from "./operation-types";

export type PreviewKind =
  | "markdown"
  | "word"
  | "code"
  | "spreadsheet"
  | "image"
  | "pdf"
  | "notebook"
  | "folder"
  | "text";

export function getPreviewLines(value: unknown, max_lines: number): string[] {
  if (value == null) {
    return [];
  }

  const text = typeof value === "string"
    ? value
    : safeJsonStringify(value);

  return text
    .split(/\r?\n/)
    .map((line) => line.trimEnd())
    .filter(Boolean)
    .slice(0, max_lines);
}

export function detectPreviewKind(target?: string | null): PreviewKind {
  if (!target) {
    return "text";
  }
  const normalized = target.toLowerCase().split("?")[0] ?? "";
  if (normalized.endsWith("/")) {
    return "folder";
  }
  const extension = normalized.includes(".")
    ? normalized.slice(normalized.lastIndexOf(".") + 1)
    : "";

  if (["md", "mdx", "markdown"].includes(extension)) {
    return "markdown";
  }
  if (["doc", "docx", "rtf", "odt"].includes(extension)) {
    return "word";
  }
  if (["csv", "tsv", "xls", "xlsx"].includes(extension)) {
    return "spreadsheet";
  }
  if (["png", "jpg", "jpeg", "gif", "webp", "svg"].includes(extension)) {
    return "image";
  }
  if (extension === "pdf") {
    return "pdf";
  }
  if (extension === "ipynb") {
    return "notebook";
  }
  if ([
    "ts",
    "tsx",
    "js",
    "jsx",
    "go",
    "py",
    "rs",
    "java",
    "css",
    "scss",
    "html",
    "json",
    "yaml",
    "yml",
    "toml",
    "sh",
    "sql",
  ].includes(extension)) {
    return "code";
  }
  return "text";
}

export function buildEditorPreviewLines(
  _event: NexusOperationEvent,
  lines: string[],
): string[] {
  if (lines.length) {
    return lines;
  }

  return [""];
}

export function basename(value?: string | null): string {
  if (!value) {
    return "target";
  }
  const normalized = value.split("/").filter(Boolean);
  return normalized.at(-1) ?? value;
}

export function formatOperationTime(timestamp: number): string {
  const normalized = timestamp < 1_000_000_000_000 ? timestamp * 1000 : timestamp;
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(normalized);
}

export function safeJsonStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}
