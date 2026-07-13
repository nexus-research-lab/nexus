import type { WorkspaceActivityItem } from "@/types/app/workspace-live";

export function resolveFinderSelectedItem(
  items: WorkspaceActivityItem[],
  selected_path: string,
): WorkspaceActivityItem | null {
  return items.find((item) => item.path === selected_path)
    ?? items.find((item) => selected_path.startsWith(`${item.path}/`))
    ?? items.find((item) => item.path.startsWith(`${selected_path}/`))
    ?? items[0]
    ?? null;
}

export function finderFileKindLabel(path: string): string {
  if (/\.(tsx?|jsx?)$/i.test(path)) {
    return "JavaScript 源代码";
  }
  if (/\.(go|rs|py|java|sh)$/i.test(path)) {
    return "源代码";
  }
  if (/\.(html?|css|scss)$/i.test(path)) {
    return "网页文件";
  }
  if (/\.(json|ya?ml|toml)$/i.test(path)) {
    return "配置文件";
  }
  if (/\.(md|mdx|txt)$/i.test(path)) {
    return "文本文稿";
  }
  if (/\.(csv|xlsx?|ods)$/i.test(path)) {
    return "电子表格";
  }
  if (/\.(png|jpe?g|webp|gif|svg)$/i.test(path)) {
    return "图像";
  }
  return path.endsWith("/") ? "文件夹" : "文件";
}

export function finderPreviewLines(item: WorkspaceActivityItem | null, max_lines = 7): string[] {
  if (!item?.live_content) {
    return [];
  }
  return item.live_content
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .split("\n")
    .map((line) => line.trimEnd())
    .filter((line) => line.trim())
    .slice(0, max_lines);
}
