import type { StageWindowState } from "../operation-desktop-types";
import { basename } from "../operation-scene-planner-helpers";

export interface StageDesktopIconItem {
  aria_label: string;
  extension_label: string;
  file_kind_label: string;
  label: string;
  state_label: string;
  target: string;
  title: string;
  window: StageWindowState;
}

export function buildStageDesktopIconItems(windows: StageWindowState[]): StageDesktopIconItem[] {
  return windows
    .filter(is_desktop_artifact_window)
    .slice(0, 5)
    .map((window) => {
      const target = window.target ?? window.payload.target ?? "";
      const label = desktop_icon_label(window);
      const extension_label = desktop_file_extension_label(target);
      const file_kind_label = desktop_file_kind_label(window);
      const state_label = desktop_file_state_label(window);
      return {
        aria_label: `${desktop_icon_action_label(window)}：${label}`,
        extension_label,
        file_kind_label,
        label,
        state_label,
        target,
        title: `${label} · ${file_kind_label} · ${state_label}`,
        window,
      };
    });
}

function is_desktop_artifact_window(window: StageWindowState): boolean {
  const target = window.target ?? window.payload.target;
  if (!target || basename(target) === "preview") {
    return false;
  }
  if (window.phase === "focused") {
    return false;
  }
  return (
    window.kind === "code_editor" ||
    window.kind === "markdown_reader" ||
    window.kind === "word_reader" ||
    window.kind === "presentation" ||
    window.kind === "pdf_reader" ||
    window.kind === "spreadsheet" ||
    window.kind === "image_viewer"
  );
}

function desktop_icon_label(window: StageWindowState): string {
  const target_label = basename(window.target ?? window.payload.target);
  if (target_label && target_label !== "preview") {
    return target_label;
  }
  return window.title;
}

function desktop_file_extension_label(target: string): string {
  const match = /\.([a-z0-9]+)$/i.exec(basename(target));
  return match?.[1]?.slice(0, 5).toUpperCase() ?? "FILE";
}

function desktop_file_kind_label(window: StageWindowState): string {
  const target = window.target ?? window.payload.target ?? "";
  if (/\.(html?|tsx?|jsx?|css|json|ya?ml|toml)$/i.test(target)) {
    return "代码文件";
  }
  if (/\.(md|mdx|txt)$/i.test(target)) {
    return "文稿";
  }
  if (/\.(png|jpe?g|webp|gif|svg)$/i.test(target)) {
    return "图像";
  }
  if (/\.(csv|xlsx?|ods)$/i.test(target)) {
    return "表格";
  }
  if (/\.pdf$/i.test(target)) {
    return "PDF";
  }
  return "文件";
}

function desktop_file_state_label(window: StageWindowState): string {
  if (window.phase === "closed") {
    return "窗口已关闭";
  }
  if (window.phase === "minimized") {
    return "窗口已最小化";
  }
  if (window.phase === "focused") {
    return "正在前台查看";
  }
  return "窗口已打开";
}

function desktop_icon_action_label(window: StageWindowState): string {
  if (window.phase === "closed" || window.phase === "minimized") {
    return "恢复文件窗口";
  }
  return "查看文件窗口";
}
