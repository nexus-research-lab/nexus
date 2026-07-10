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
  | "binary"
  | "unknown";

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
]);

const EXTENSION_PREVIEW_KINDS = new Map<string, WorkspaceFilePreviewKind>([
  ["pdf", "pdf"],
  ["xlsx", "spreadsheet"],
  ["docx", "document"],
  ["pptx", "presentation"],
  ["md", "markdown"],
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
  const ext = path.split(".").pop()?.toLowerCase() || "";
  return EXTENSION_PREVIEW_KINDS.get(ext) ?? "binary";
}

export function workspaceFileKindLabel(
  fileType: WorkspaceFilePreviewKind,
): string {
  return WORKSPACE_FILE_KIND_LABELS[fileType] ?? "文件预览";
}
