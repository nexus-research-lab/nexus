import type { ArtifactKind } from "@/types/conversation/artifact";

const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "webp", "gif", "avif"]);

export function file_name_from_artifact_path(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  return normalized.split("/").filter(Boolean).at(-1) ?? normalized;
}

export function artifact_kind_from_path(path: string, mime_type?: string | null): ArtifactKind {
  const normalized_mime_type = mime_type?.trim().toLowerCase() ?? "";
  if (normalized_mime_type.startsWith("image/")) {
    return normalized_mime_type === "image/svg+xml" ? "svg" : "image";
  }
  if (normalized_mime_type === "application/pdf") {
    return "pdf";
  }
  if (normalized_mime_type === "text/html") {
    return "html";
  }
  if (normalized_mime_type === "text/markdown") {
    return "markdown";
  }

  const extension = file_name_from_artifact_path(path).split(".").pop()?.toLowerCase() ?? "";
  if (IMAGE_EXTENSIONS.has(extension)) {
    return "image";
  }
  if (extension === "svg") {
    return "svg";
  }
  if (extension === "pdf") {
    return "pdf";
  }
  if (extension === "md" || extension === "markdown") {
    return "markdown";
  }
  if (extension === "html" || extension === "htm") {
    return "html";
  }
  if (extension === "mmd" || extension === "mermaid") {
    return "mermaid";
  }
  return extension ? "file" : "unknown";
}

export function normalize_artifact_path(value: string): string {
  const normalized = value
    .trim()
    .replace(/%60/gi, "`")
    .replace(/\\/g, "/")
    .replace(/^[("'`【]+|[)"'`】,，。；：:!?]+$/g, "");
  const workspace_match = /\/\.nexus\/workspace\/[^/\s`"'，。；！？]+\/(?<relative>[^\s`"'，。；！？]+)/.exec(normalized);
  if (workspace_match?.groups?.relative) {
    return workspace_match.groups.relative.replace(/^\.\//, "");
  }
  return normalized.replace(/^\.\//, "");
}

export function is_workspace_artifact_reference(value: string): boolean {
  const normalized = normalize_artifact_path(value);
  if (!normalized || /^(https?:|data:|blob:)/i.test(normalized)) {
    return false;
  }
  if (normalized.startsWith("/") && !normalized.includes("/.nexus/workspace/")) {
    return false;
  }
  return /[A-Za-z0-9_.-][A-Za-z0-9_./-]*\.[A-Za-z0-9]{1,10}$/.test(normalized);
}

export function artifact_kind_label(kind: ArtifactKind): string {
  switch (kind) {
    case "image":
      return "图片";
    case "svg":
      return "SVG";
    case "pdf":
      return "PDF";
    case "markdown":
      return "Markdown";
    case "html":
      return "HTML";
    case "mermaid":
      return "Mermaid";
    case "code":
      return "代码";
    case "file":
      return "文件";
    default:
      return "产物";
  }
}
