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
  icon_class_name: string;
}

function get_file_extension(name: string): string | null {
  const lower_name = name.toLowerCase();
  if (lower_name === "dockerfile") {
    return "docker";
  }
  if (lower_name === "makefile") {
    return "make";
  }

  const last_dot_index = lower_name.lastIndexOf(".");
  if (last_dot_index <= 0 || last_dot_index === lower_name.length - 1) {
    return null;
  }
  return lower_name.slice(last_dot_index + 1);
}

/** 中文注释：文件图标映射独立成纯函数，避免视图文件继续承载规则表。 */
export function get_workspace_file_visual(name: string): WorkspaceFileVisual {
  const extension = get_file_extension(name);

  if (!extension) {
    return {
      Icon: FileText,
      icon_class_name: "text-(--icon-muted)",
    };
  }

  if (IMAGE_EXTENSIONS.has(extension)) {
    return {
      Icon: Image,
      icon_class_name: "text-[color:color-mix(in_srgb,var(--primary)_68%,var(--destructive)_32%)]",
    };
  }

  if (ARCHIVE_EXTENSIONS.has(extension)) {
    return {
      Icon: FileArchive,
      icon_class_name: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
    };
  }

  if (SPREADSHEET_EXTENSIONS.has(extension)) {
    return {
      Icon: FileSpreadsheet,
      icon_class_name: "text-(--success)",
    };
  }

  if (JSON_EXTENSIONS.has(extension)) {
    return {
      Icon: FileJson,
      icon_class_name: "text-(--success)",
    };
  }

  if (WEB_CODE_EXTENSIONS.has(extension)) {
    return {
      Icon: FileCode2,
      icon_class_name: "text-(--primary)",
    };
  }

  if (SCRIPT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileCode2,
      icon_class_name: extension === "py" ? "text-(--warning)" : "text-(--primary)",
    };
  }

  if (CONFIG_EXTENSIONS.has(extension)) {
    return {
      Icon: FileText,
      icon_class_name: "text-(--accent)",
    };
  }

  if (TEXT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileText,
      icon_class_name: extension === "md" || extension === "markdown" ? "text-(--primary)" : "text-(--icon-muted)",
    };
  }

  if (DOCUMENT_EXTENSIONS.has(extension)) {
    return {
      Icon: FileType2,
      icon_class_name: extension === "pdf" ? "text-(--destructive)" : "text-(--warning)",
    };
  }

  return {
    Icon: File,
    icon_class_name: "text-(--icon-muted)",
  };
}
