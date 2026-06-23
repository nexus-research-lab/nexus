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

const text_extensions = new Set([
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

const image_extensions = new Set([
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

export function get_workspace_file_preview_kind(
  path: string,
): WorkspaceFilePreviewKind {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (ext === "pdf") {
    return "pdf";
  }
  if (image_extensions.has(ext)) {
    return "image";
  }
  if (ext === "xlsx") {
    return "spreadsheet";
  }
  if (ext === "docx") {
    return "document";
  }
  if (ext === "pptx") {
    return "presentation";
  }
  if (ext === "md" || ext === "markdown") {
    return "markdown";
  }
  if (ext === "html" || ext === "htm") {
    return "html";
  }
  if (ext === "mmd" || ext === "mermaid") {
    return "mermaid";
  }
  if (text_extensions.has(ext)) {
    return "text";
  }
  return "binary";
}

export function is_workspace_text_preview_kind(
  kind: WorkspaceFilePreviewKind,
): boolean {
  return (
    kind === "text" ||
    kind === "markdown" ||
    kind === "html" ||
    kind === "mermaid"
  );
}

export function workspace_file_kind_label(
  file_type: WorkspaceFilePreviewKind,
): string {
  switch (file_type) {
    case "markdown":
      return "Markdown 预览";
    case "html":
      return "HTML 预览";
    case "mermaid":
      return "Mermaid 预览";
    case "text":
      return "文本预览";
    default:
      return "文件预览";
  }
}
