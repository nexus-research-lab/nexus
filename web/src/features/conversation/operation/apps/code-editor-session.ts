export interface CodeEditorSessionView {
  cursor_label: string;
  extension_label: string;
  is_code: boolean;
  language_label: string;
  line_count: number;
  status_label: string;
  tab_title: string;
}

const LANGUAGE_LABELS: Record<string, string> = {
  css: "CSS",
  go: "Go",
  html: "HTML",
  js: "JavaScript",
  json: "JSON",
  jsx: "JavaScript React",
  md: "Markdown",
  py: "Python",
  rs: "Rust",
  scss: "SCSS",
  sh: "Shell",
  sql: "SQL",
  toml: "TOML",
  ts: "TypeScript",
  tsx: "TypeScript React",
  txt: "Plain Text",
  yaml: "YAML",
  yml: "YAML",
};

export function buildCodeEditorSessionView({
  diff_stats,
  lines,
  title,
}: {
  diff_stats?: { additions: number; deletions: number } | null;
  lines: string[];
  title: string;
}): CodeEditorSessionView {
  const extension = file_extension(title);
  const language_label = LANGUAGE_LABELS[extension] ?? (extension ? extension.toUpperCase() : "Plain Text");
  const is_code = language_label !== "Plain Text";
  const line_count = Math.max(lines.length, 1);
  const change_label = diff_stats
    ? ` · +${diff_stats.additions} -${diff_stats.deletions}`
    : "";

  return {
    cursor_label: `Ln ${line_count}, Col 1`,
    extension_label: extension ? extension.toUpperCase() : "TEXT",
    is_code,
    language_label,
    line_count,
    status_label: `UTF-8 · Spaces: 2 · ${language_label}${change_label}`,
    tab_title: compact_editor_title(title),
  };
}

function file_extension(title: string): string {
  const file_name = title.split(/[\\/]/).filter(Boolean).at(-1) ?? title;
  if (!file_name.includes(".")) {
    return "";
  }
  return file_name.slice(file_name.lastIndexOf(".") + 1).toLowerCase();
}

function compact_editor_title(title: string): string {
  const normalized = title.trim() || "Untitled";
  return normalized.length > 26 ? `${normalized.slice(0, 25)}…` : normalized;
}
