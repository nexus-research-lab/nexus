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

const TEXT_FILE_LANGUAGES = new Map<string, string | null>([
  ["txt", null],
  ["log", null],
  ["json", "json"],
  ["jsonl", "json"],
  ["yaml", "yaml"],
  ["yml", "yaml"],
  ["toml", "toml"],
  ["xml", "markup"],
  ["csv", "csv"],
  ["ts", "typescript"],
  ["tsx", "tsx"],
  ["js", "javascript"],
  ["jsx", "jsx"],
  ["mjs", "javascript"],
  ["cjs", "javascript"],
  ["py", "python"],
  ["java", "java"],
  ["go", "go"],
  ["rs", "rust"],
  ["rb", "ruby"],
  ["php", "php"],
  ["sh", "bash"],
  ["bash", "bash"],
  ["zsh", "bash"],
  ["sql", "sql"],
  ["r", "r"],
  ["css", "css"],
  ["scss", "scss"],
  ["less", "less"],
  ["ini", "ini"],
  ["conf", "ini"],
  ["env", "bash"],
  ["dockerfile", "docker"],
  ["makefile", "makefile"],
  ["cmake", "cmake"],
  ["gradle", "groovy"],
  ["proto", "protobuf"],
  ["graphql", "graphql"],
  ["rst", "rest"],
  ["adoc", "asciidoc"],
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
for (const extension of TEXT_FILE_LANGUAGES.keys()) {
  EXTENSION_PREVIEW_KINDS.set(extension, "text");
}

export function getWorkspaceFilePreviewKind(
  path: string,
): WorkspaceFilePreviewKind {
  const ext = workspaceFileExtension(path);
  return EXTENSION_PREVIEW_KINDS.get(ext) ?? "binary";
}

export function getWorkspaceFileCodeLanguage(path: string): string | null {
  return TEXT_FILE_LANGUAGES.get(workspaceFileExtension(path)) ?? null;
}

function workspaceFileExtension(path: string): string {
  const fileName = path.split("/").pop()?.toLowerCase() ?? "";
  if (fileName.startsWith(".env")) {
    return "env";
  }
  if (fileName.startsWith("dockerfile")) {
    return "dockerfile";
  }
  return fileName.split(".").pop() ?? "";
}
