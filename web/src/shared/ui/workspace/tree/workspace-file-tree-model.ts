import archiveIconSrc from "material-icon-theme/icons/zip.svg";
import audioIconSrc from "material-icon-theme/icons/audio.svg";
import cIconSrc from "material-icon-theme/icons/c.svg";
import configIconSrc from "material-icon-theme/icons/settings.svg";
import cppIconSrc from "material-icon-theme/icons/cpp.svg";
import cssIconSrc from "material-icon-theme/icons/css.svg";
import csharpIconSrc from "material-icon-theme/icons/csharp.svg";
import dartIconSrc from "material-icon-theme/icons/dart.svg";
import databaseIconSrc from "material-icon-theme/icons/database.svg";
import dockerIconSrc from "material-icon-theme/icons/docker.svg";
import fileIconSrc from "material-icon-theme/icons/file.svg";
import folderIconSrc from "material-icon-theme/icons/folder.svg";
import folderOpenIconSrc from "material-icon-theme/icons/folder-open.svg";
import fontIconSrc from "material-icon-theme/icons/font.svg";
import goIconSrc from "material-icon-theme/icons/go.svg";
import goModuleIconSrc from "material-icon-theme/icons/go-mod.svg";
import graphqlIconSrc from "material-icon-theme/icons/graphql.svg";
import htmlIconSrc from "material-icon-theme/icons/html.svg";
import imageIconSrc from "material-icon-theme/icons/image.svg";
import javaIconSrc from "material-icon-theme/icons/java.svg";
import javascriptIconSrc from "material-icon-theme/icons/javascript.svg";
import jsonIconSrc from "material-icon-theme/icons/json.svg";
import kotlinIconSrc from "material-icon-theme/icons/kotlin.svg";
import lessIconSrc from "material-icon-theme/icons/less.svg";
import lockIconSrc from "material-icon-theme/icons/lock.svg";
import logIconSrc from "material-icon-theme/icons/log.svg";
import luaIconSrc from "material-icon-theme/icons/lua.svg";
import makefileIconSrc from "material-icon-theme/icons/makefile.svg";
import markdownIconSrc from "material-icon-theme/icons/markdown.svg";
import npmIconSrc from "material-icon-theme/icons/npm.svg";
import pdfIconSrc from "material-icon-theme/icons/pdf.svg";
import perlIconSrc from "material-icon-theme/icons/perl.svg";
import phpIconSrc from "material-icon-theme/icons/php.svg";
import pnpmIconSrc from "material-icon-theme/icons/pnpm.svg";
import powerpointIconSrc from "material-icon-theme/icons/powerpoint.svg";
import protoIconSrc from "material-icon-theme/icons/proto.svg";
import pythonIconSrc from "material-icon-theme/icons/python.svg";
import rIconSrc from "material-icon-theme/icons/r.svg";
import reactIconSrc from "material-icon-theme/icons/react.svg";
import reactTypescriptIconSrc from "material-icon-theme/icons/react_ts.svg";
import rubyIconSrc from "material-icon-theme/icons/ruby.svg";
import rustIconSrc from "material-icon-theme/icons/rust.svg";
import sassIconSrc from "material-icon-theme/icons/sass.svg";
import scalaIconSrc from "material-icon-theme/icons/scala.svg";
import shellIconSrc from "material-icon-theme/icons/console.svg";
import swiftIconSrc from "material-icon-theme/icons/swift.svg";
import tableIconSrc from "material-icon-theme/icons/table.svg";
import textIconSrc from "material-icon-theme/icons/document.svg";
import tomlIconSrc from "material-icon-theme/icons/toml.svg";
import tsconfigIconSrc from "material-icon-theme/icons/tsconfig.svg";
import typescriptIconSrc from "material-icon-theme/icons/typescript.svg";
import videoIconSrc from "material-icon-theme/icons/video.svg";
import viteIconSrc from "material-icon-theme/icons/vite.svg";
import wordIconSrc from "material-icon-theme/icons/word.svg";
import xmlIconSrc from "material-icon-theme/icons/xml.svg";
import yamlIconSrc from "material-icon-theme/icons/yaml.svg";

import { cn } from "@/shared/ui/class-name";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

export interface WorkspaceFileTreeNode {
  children: WorkspaceFileTreeNode[];
  entry: WorkspaceFileEntry;
}

interface WorkspaceFileVisual {
  iconSrc: string;
}

interface WorkspaceFileVisualNameRule extends WorkspaceFileVisual {
  pattern: RegExp;
}

interface WorkspaceFileTreeRowPresentation {
  actionsVisible: boolean;
  chevronClassName: string;
  isDirectoryTarget: boolean;
  isSelected: boolean;
  nameClassName: string;
  paddingLeft: number;
  rowClassName: string;
  showChildren: boolean;
}

const DEFAULT_FILE_VISUAL: WorkspaceFileVisual = {
  iconSrc: fileIconSrc,
};

const NO_EXTENSION_VISUAL: WorkspaceFileVisual = {
  iconSrc: textIconSrc,
};

function buildExtensionVisual(
  iconSrc: string,
  extensions: readonly string[],
): Array<readonly [string, WorkspaceFileVisual]> {
  const visual = { iconSrc };
  return extensions.map((extension) => [extension, visual] as const);
}

const FILE_VISUAL_NAME_RULES: WorkspaceFileVisualNameRule[] = [
  { iconSrc: npmIconSrc, pattern: /^package(?:-lock)?\.json$/ },
  { iconSrc: pnpmIconSrc, pattern: /^pnpm-(?:lock|workspace)\.ya?ml$/ },
  { iconSrc: tsconfigIconSrc, pattern: /^(?:js|ts)config(?:\..+)?\.json$/ },
  { iconSrc: viteIconSrc, pattern: /^vite\.config\./ },
  { iconSrc: dockerIconSrc, pattern: /^dockerfile(?:\..+)?$/ },
  { iconSrc: makefileIconSrc, pattern: /^makefile(?:\..+)?$/ },
  { iconSrc: goModuleIconSrc, pattern: /^go\.(?:mod|sum)$/ },
  { iconSrc: markdownIconSrc, pattern: /^readme(?:\..+)?$/ },
  { iconSrc: configIconSrc, pattern: /^\.env(?:\..+)?$/ },
];

const FILE_VISUAL_BY_EXTENSION = new Map<string, WorkspaceFileVisual>([
  ...buildExtensionVisual(imageIconSrc, [
    "png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif",
  ]),
  ...buildExtensionVisual(archiveIconSrc, [
    "zip", "tar", "gz", "rar", "7z", "bz2", "xz",
  ]),
  ...buildExtensionVisual(tableIconSrc, ["xlsx", "xls", "csv", "ods"]),
  ...buildExtensionVisual(jsonIconSrc, ["json", "jsonl"]),
  ...buildExtensionVisual(htmlIconSrc, ["html", "htm"]),
  ...buildExtensionVisual(javascriptIconSrc, ["js", "mjs", "cjs"]),
  ...buildExtensionVisual(typescriptIconSrc, ["ts", "mts", "cts"]),
  ...buildExtensionVisual(reactIconSrc, ["jsx"]),
  ...buildExtensionVisual(reactTypescriptIconSrc, ["tsx"]),
  ...buildExtensionVisual(pythonIconSrc, ["py", "pyw"]),
  ...buildExtensionVisual(goIconSrc, ["go"]),
  ...buildExtensionVisual(rustIconSrc, ["rs"]),
  ...buildExtensionVisual(javaIconSrc, ["java", "class", "jar"]),
  ...buildExtensionVisual(cIconSrc, ["c", "h"]),
  ...buildExtensionVisual(cppIconSrc, ["cc", "cpp", "cxx", "hpp"]),
  ...buildExtensionVisual(csharpIconSrc, ["cs"]),
  ...buildExtensionVisual(swiftIconSrc, ["swift"]),
  ...buildExtensionVisual(kotlinIconSrc, ["kt", "kts"]),
  ...buildExtensionVisual(dartIconSrc, ["dart"]),
  ...buildExtensionVisual(phpIconSrc, ["php"]),
  ...buildExtensionVisual(rubyIconSrc, ["rb"]),
  ...buildExtensionVisual(luaIconSrc, ["lua"]),
  ...buildExtensionVisual(perlIconSrc, ["pl", "pm"]),
  ...buildExtensionVisual(rIconSrc, ["r"]),
  ...buildExtensionVisual(scalaIconSrc, ["scala", "sc"]),
  ...buildExtensionVisual(shellIconSrc, ["sh", "bash", "zsh", "fish"]),
  ...buildExtensionVisual(databaseIconSrc, ["sql", "db", "sqlite", "sqlite3"]),
  ...buildExtensionVisual(graphqlIconSrc, ["graphql", "gql"]),
  ...buildExtensionVisual(protoIconSrc, ["proto"]),
  ...buildExtensionVisual(cssIconSrc, ["css"]),
  ...buildExtensionVisual(sassIconSrc, ["sass", "scss"]),
  ...buildExtensionVisual(lessIconSrc, ["less"]),
  ...buildExtensionVisual(yamlIconSrc, ["yaml", "yml"]),
  ...buildExtensionVisual(tomlIconSrc, ["toml"]),
  ...buildExtensionVisual(xmlIconSrc, ["xml"]),
  ...buildExtensionVisual(configIconSrc, ["ini", "conf", "cfg", "env"]),
  ...buildExtensionVisual(markdownIconSrc, ["md", "markdown", "mdx"]),
  ...buildExtensionVisual(textIconSrc, ["txt", "rst", "adoc"]),
  ...buildExtensionVisual(logIconSrc, ["log"]),
  ...buildExtensionVisual(lockIconSrc, ["lock"]),
  ...buildExtensionVisual(pdfIconSrc, ["pdf"]),
  ...buildExtensionVisual(wordIconSrc, ["doc", "docx", "odt", "rtf"]),
  ...buildExtensionVisual(powerpointIconSrc, ["ppt", "pptx", "odp"]),
  ...buildExtensionVisual(audioIconSrc, ["mp3", "wav", "ogg", "flac", "aac", "m4a"]),
  ...buildExtensionVisual(videoIconSrc, ["mp4", "mov", "webm", "mkv", "avi"]),
  ...buildExtensionVisual(fontIconSrc, ["ttf", "otf", "woff", "woff2"]),
]);

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
  const nameRule = FILE_VISUAL_NAME_RULES.find(({ pattern }) => (
    pattern.test(normalizedName)
  ));
  if (nameRule) {
    return { iconSrc: nameRule.iconSrc };
  }
  const extension = getFileExtension(normalizedName);
  if (!extension) {
    return NO_EXTENSION_VISUAL;
  }
  return FILE_VISUAL_BY_EXTENSION.get(extension) ?? DEFAULT_FILE_VISUAL;
}

export function getWorkspaceDirectoryIcon(isOpen: boolean): string {
  return isOpen ? folderOpenIconSrc : folderIconSrc;
}

export function getWorkspaceFileTreeRowPresentation({
  activePath,
  depth,
  entry,
  focusedDirectoryPath,
  isOpen,
}: {
  activePath: string | null;
  depth: number;
  entry: WorkspaceFileEntry;
  focusedDirectoryPath: string | null;
  isOpen: boolean;
}): WorkspaceFileTreeRowPresentation {
  const isActive = entry.path === activePath;
  const isDirectoryTarget = entry.is_dir && entry.path === focusedDirectoryPath;
  const isSelected = !entry.is_dir && isActive;
  return {
    actionsVisible: isSelected,
    chevronClassName: cn(
      "h-3 w-3 shrink-0 transition-transform",
      isSelected ? "text-(--icon-default)" : "text-(--icon-muted)",
      isOpen && "rotate-90",
    ),
    isDirectoryTarget,
    isSelected,
    nameClassName: cn(
      "shrink-0 whitespace-nowrap text-sm leading-[1.3rem]",
      entry.is_dir || isSelected ? "font-medium" : "font-normal",
    ),
    paddingLeft: 8 + depth * 12,
    rowClassName: cn(
      "group relative flex min-w-full w-max items-center radius-control-md pr-2 text-left transition-colors",
      isSelected
        ? "bg-(--surface-sidebar-active-background) text-(--text-strong)"
        : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    ),
    showChildren: entry.is_dir && isOpen,
  };
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
