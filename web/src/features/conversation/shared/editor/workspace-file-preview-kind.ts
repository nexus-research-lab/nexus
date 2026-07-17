/**
 * INPUT: A workspace file path.
 * OUTPUT: The concrete shared preview renderer kind and its user-facing label.
 * POS: Single extension-to-preview registry shared by workspace and Operation Stage adapters.
 */
export type WorkspaceFilePreviewKind =
  | "text"
  | "markdown"
  | "html"
  | "mermaid"
  | "pdf"
  | "image"
  | "spreadsheet"
  | "document"
  | "presentation"
  | "binary";

const textExtensions = new Set([
  "txt",
  "json",
  "jsonl",
  "yaml",
  "yml",
  "toml",
  "xml",
  "csv",
  "ts",
  "tsx",
  "js",
  "jsx",
  "mjs",
  "cjs",
  "py",
  "java",
  "go",
  "rs",
  "rb",
  "php",
  "sh",
  "bash",
  "zsh",
  "sql",
  "r",
  "css",
  "scss",
  "less",
  "log",
  "ini",
  "conf",
  "env",
  "dockerfile",
  "makefile",
  "cmake",
  "gradle",
  "proto",
  "graphql",
  "rst",
  "adoc",
  "c",
  "cc",
  "cpp",
  "h",
  "hpp",
  "swift",
  "kt",
  "kts",
  "lua",
  "vue",
  "svelte",
  "dart",
  "cs",
  "fs",
  "fsx",
  "ex",
  "exs",
  "erl",
  "hrl",
  "ps1",
  "ipynb",
  "mod",
  "sum",
  "lock",
]);

const imageExtensions = new Set([
  "png",
  "jpg",
  "jpeg",
  "gif",
  "webp",
  "svg",
  "bmp",
  "ico",
  "avif",
  "tif",
  "tiff",
]);

const EXTENSION_PREVIEW_KINDS = new Map<string, WorkspaceFilePreviewKind>([
  ["pdf", "pdf"],
  ["xlsx", "spreadsheet"],
  ["docx", "document"],
  ["pptx", "presentation"],
  ["md", "markdown"],
  ["mdx", "markdown"],
  ["markdown", "markdown"],
  ["html", "html"],
  ["htm", "html"],
  ["mmd", "mermaid"],
  ["mermaid", "mermaid"],
]);
for (const extension of imageExtensions) {
  EXTENSION_PREVIEW_KINDS.set(extension, "image");
}
for (const extension of textExtensions) {
  EXTENSION_PREVIEW_KINDS.set(extension, "text");
}

const WORKSPACE_FILE_KIND_LABELS: Partial<Record<
  WorkspaceFilePreviewKind,
  string
>> = {
  html: "HTML 预览",
  markdown: "Markdown 预览",
  mermaid: "Mermaid 预览",
  text: "文本预览",
};

export function getWorkspaceFilePreviewKind(
  path: string,
): WorkspaceFilePreviewKind {
  const fileName = path.replace(/\\/g, "/").split("/").at(-1)?.toLowerCase() ?? "";
  if (fileName.startsWith(".env")) {
    return "text";
  }
  const ext = fileName.split(".").pop() || "";
  return EXTENSION_PREVIEW_KINDS.get(ext) ?? "binary";
}

export function workspaceFileKindLabel(
  fileType: WorkspaceFilePreviewKind,
): string {
  return WORKSPACE_FILE_KIND_LABELS[fileType] ?? "文件预览";
}
