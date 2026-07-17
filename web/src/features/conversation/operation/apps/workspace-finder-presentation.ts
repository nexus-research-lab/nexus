/**
 * INPUT: Workspace file paths, sizes, and modification timestamps.
 * OUTPUT: Stable visual metadata used by the Agent OS Files app.
 * POS: Pure Files presentation helpers; no React state or workspace I/O.
 */
import {
  FileCode2,
  FileImage,
  FileSpreadsheet,
  FileText,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export function workspaceFileIcon(path: string): LucideIcon {
  if (/\.(tsx?|jsx?|json|ya?ml|toml|css|scss|html?)$/i.test(path)) {
    return FileCode2;
  }
  if (/\.(csv|xlsx?|ods)$/i.test(path)) {
    return FileSpreadsheet;
  }
  if (/\.(png|jpe?g|webp|gif|svg)$/i.test(path)) {
    return FileImage;
  }
  return FileText;
}

export function formatWorkspaceFileTime(timestamp?: string | number | null): string {
  if (timestamp == null || timestamp === "") {
    return "--";
  }
  const value = typeof timestamp === "number" && timestamp < 1_000_000_000_000
    ? timestamp * 1000
    : timestamp;
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}
