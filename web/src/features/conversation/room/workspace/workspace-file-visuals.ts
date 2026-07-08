/**
 * =====================================================
 * @File   : workspace-file-visuals.ts
 * @Date   : 2026-04-15 17:41
 * @Author : leemysw
 * 2026-04-15 17:41   Create
 * =====================================================
 */

import {
  File,
  FileArchive,
  FileCode2,
  FileJson,
  FileSpreadsheet,
  FileText,
  FileType2,
  Image,
  type LucideIcon,
} from "lucide-react";

const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif"]);
const ARCHIVE_EXTENSIONS = new Set(["zip", "tar", "gz", "rar", "7z", "bz2", "xz"]);
const SPREADSHEET_EXTENSIONS = new Set(["xlsx", "xls", "csv", "ods"]);
const JSON_EXTENSIONS = new Set(["json", "jsonl"]);
const WEB_CODE_EXTENSIONS = new Set(["ts", "tsx", "js", "jsx", "mjs", "cjs", "html", "css", "scss", "less", "sass", "styl"]);
const SCRIPT_EXTENSIONS = new Set(["py", "go", "rs", "java", "c", "cpp", "h", "hpp", "cs", "swift", "kt", "dart", "php", "rb", "sh", "bash", "zsh", "sql", "r", "scala", "groovy", "lua", "pl", "perl"]);
const CONFIG_EXTENSIONS = new Set(["yaml", "yml", "toml", "ini", "conf", "env", "xml", "graphql", "proto"]);
const TEXT_EXTENSIONS = new Set(["md", "markdown", "txt", "log"]);
const DOCUMENT_EXTENSIONS = new Set(["pdf", "doc", "docx", "ppt", "pptx", "odt", "rtf"]);

export interface WorkspaceFileVisual {
  Icon: LucideIcon;
  iconClassName: string;
}

function getFileExtension(name: string): string | null {
  const lowerName = name.toLowerCase();
  if (lowerName === "dockerfile") {
    return "docker";
  }
  if (lowerName === "makefile") {
    return "make";
  }

  const lastDotIndex = lowerName.lastIndexOf(".");
  if (lastDotIndex <= 0 || lastDotIndex === lowerName.length - 1) {
    return null;
  }
  return lowerName.slice(lastDotIndex + 1);
}

/** 中文注释：文件图标映射独立成纯函数，避免视图文件继续承载规则表。 */
export function getWorkspaceFileVisual(name: string): WorkspaceFileVisual {
  const extension = getFileExtension(name);

  if (!extension) {
    return {
      Icon: FileText,
      iconClassName: "text-(--icon-muted)",
    };
  }

  if (IMAGE_EXTENSIONS.has(extension)) {
    return {
      Icon: Image,
      iconClassName: "text-[color:color-mix(in_srgb,var(--primary)_68%,var(--destructive)_32%)]",
    };
  }

  if (ARCHIVE_EXTENSIONS.has(extension)) {
    return {
      Icon: FileArchive,
      iconClassName: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
    };
  }

  if (SPREADSHEET_EXTENSIONS.has(extension)) {
    return {
      Icon: FileSpreadsheet,
      iconClassName: "text-(--success)",
    };
  }

  if (JSON_EXTENSIONS.has(extension)) {
    return {
      Icon: FileJson,
      iconClassName: "text-(--success)",
    };
  }

  if (WEB_CODE_EXTENSIONS.has(extension)) {
    return {
      Icon: FileCode2,
      iconClassName: "text-(--primary)",
    };
  }

  if (SCRIPT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileCode2,
      iconClassName: extension === "py" ? "text-(--warning)" : "text-(--primary)",
    };
  }

  if (CONFIG_EXTENSIONS.has(extension)) {
    return {
      Icon: FileText,
      iconClassName: "text-(--accent)",
    };
  }

  if (TEXT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileText,
      iconClassName: extension === "md" || extension === "markdown" ? "text-(--primary)" : "text-(--icon-muted)",
    };
  }

  if (DOCUMENT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileType2,
      iconClassName: extension === "pdf" ? "text-(--destructive)" : "text-(--warning)",
    };
  }

  return {
    Icon: File,
    iconClassName: "text-(--icon-muted)",
  };
}
