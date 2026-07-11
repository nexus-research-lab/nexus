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

import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceFileTreeNode {
  children: WorkspaceFileTreeNode[];
  entry: WorkspaceFileEntry;
}

interface WorkspaceFileVisual {
  Icon: LucideIcon;
  iconClassName: string;
}

interface WorkspaceFileVisualGroup extends WorkspaceFileVisual {
  extensions: readonly string[];
}

const DEFAULT_FILE_VISUAL: WorkspaceFileVisual = {
  Icon: File,
  iconClassName: "text-(--icon-muted)",
};

const NO_EXTENSION_VISUAL: WorkspaceFileVisual = {
  Icon: FileText,
  iconClassName: "text-(--icon-muted)",
};

const FILE_VISUAL_BY_NAME = new Map<string, WorkspaceFileVisual>([
  ["dockerfile", DEFAULT_FILE_VISUAL],
  ["makefile", DEFAULT_FILE_VISUAL],
]);

const FILE_VISUAL_GROUPS: WorkspaceFileVisualGroup[] = [
  {
    Icon: Image,
    extensions: ["png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif"],
    iconClassName: "text-[color:color-mix(in_srgb,var(--primary)_68%,var(--destructive)_32%)]",
  },
  {
    Icon: FileArchive,
    extensions: ["zip", "tar", "gz", "rar", "7z", "bz2", "xz"],
    iconClassName: "text-[color:color-mix(in_srgb,var(--primary)_72%,var(--text-strong)_28%)]",
  },
  {
    Icon: FileSpreadsheet,
    extensions: ["xlsx", "xls", "csv", "ods"],
    iconClassName: "text-(--success)",
  },
  {
    Icon: FileJson,
    extensions: ["json", "jsonl"],
    iconClassName: "text-(--success)",
  },
  {
    Icon: FileCode2,
    extensions: [
      "ts", "tsx", "js", "jsx", "mjs", "cjs", "html", "css", "scss",
      "less", "sass", "styl", "go", "rs", "java", "c", "cpp", "h", "hpp",
      "cs", "swift", "kt", "dart", "php", "rb", "sh", "bash", "zsh", "sql",
      "r", "scala", "groovy", "lua", "pl", "perl",
    ],
    iconClassName: "text-(--primary)",
  },
  {
    Icon: FileCode2,
    extensions: ["py"],
    iconClassName: "text-(--warning)",
  },
  {
    Icon: FileText,
    extensions: ["yaml", "yml", "toml", "ini", "conf", "env", "xml", "graphql", "proto"],
    iconClassName: "text-(--accent)",
  },
  {
    Icon: FileText,
    extensions: ["md", "markdown"],
    iconClassName: "text-(--primary)",
  },
  {
    Icon: FileText,
    extensions: ["txt", "log"],
    iconClassName: "text-(--icon-muted)",
  },
  {
    Icon: FileType2,
    extensions: ["pdf"],
    iconClassName: "text-(--destructive)",
  },
  {
    Icon: FileType2,
    extensions: ["doc", "docx", "ppt", "pptx", "odt", "rtf"],
    iconClassName: "text-(--warning)",
  },
];

const FILE_VISUAL_BY_EXTENSION = new Map(
  FILE_VISUAL_GROUPS.flatMap(({ extensions, ...visual }) =>
    extensions.map((extension) => [extension, visual] as const),
  ),
);

export function buildWorkspaceFileTree(
  entries: WorkspaceFileEntry[],
): WorkspaceFileTreeNode[] {
  const roots: WorkspaceFileTreeNode[] = [];
  const nodeByPath = new Map<string, WorkspaceFileTreeNode>();
  const sortedEntries = [...entries].sort(compareWorkspaceEntries);

  for (const entry of sortedEntries) {
    const node = { children: [], entry };
    nodeByPath.set(entry.path, node);
    const parent = nodeByPath.get(getParentPath(entry.path));
    (parent?.children ?? roots).push(node);
  }

  return roots;
}

export function getWorkspaceFileVisual(name: string): WorkspaceFileVisual {
  const normalizedName = name.toLowerCase();
  const namedVisual = FILE_VISUAL_BY_NAME.get(normalizedName);
  if (namedVisual) {
    return namedVisual;
  }
  const extension = getFileExtension(normalizedName);
  if (!extension) {
    return NO_EXTENSION_VISUAL;
  }
  return FILE_VISUAL_BY_EXTENSION.get(extension) ?? DEFAULT_FILE_VISUAL;
}

function compareWorkspaceEntries(
  left: WorkspaceFileEntry,
  right: WorkspaceFileEntry,
): number {
  return Number(right.is_dir) - Number(left.is_dir)
    || left.path.localeCompare(right.path);
}

function getParentPath(path: string): string {
  const separatorIndex = path.lastIndexOf("/");
  return separatorIndex < 0 ? "" : path.slice(0, separatorIndex);
}

function getFileExtension(name: string): string | null {
  const lastDotIndex = name.lastIndexOf(".");
  if (lastDotIndex <= 0 || lastDotIndex === name.length - 1) {
    return null;
  }
  return name.slice(lastDotIndex + 1);
}
