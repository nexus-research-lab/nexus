export function firstNonEmptyArtifactValue(
  ...values: Array<string | null | undefined>
): string {
  return values
    .map((value) => value?.trim() ?? "")
    .find(Boolean) ?? "";
}

function normalizeArtifactPath(path: string): string {
  return path.trim().replace(/\\/g, "/");
}

export function getArtifactFileName(path: string, fallback = "文件"): string {
  const normalizedPath = normalizeArtifactPath(path);
  return normalizedPath.split("/").filter(Boolean).at(-1) ?? fallback;
}

export function getArtifactParentPath(path: string): string {
  const parts = normalizeArtifactPath(path).split("/").filter(Boolean);
  const parentParts = parts.slice(0, -1);
  return parentParts.join("/") || "workspace";
}
