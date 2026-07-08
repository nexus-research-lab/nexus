import { isDesktopRuntime } from "@/config/desktop-runtime";

export interface WorkspaceFileExternalActionCopy {
  ariaLabel: string;
  label: string;
  mode: "download" | "reveal";
  title: string;
}

export const getWorkspaceFileExternalActionCopy = (
  fileName?: string,
): WorkspaceFileExternalActionCopy => {
  const normalizedFileName = fileName?.trim() || "文件";
  if (isDesktopRuntime()) {
    return {
      ariaLabel: `在文件夹中显示 ${normalizedFileName}`,
      label: "打开",
      mode: "reveal",
      title: `在文件夹中显示 ${normalizedFileName}`,
    };
  }
  return {
    ariaLabel: `下载 ${normalizedFileName}`,
    label: "下载",
    mode: "download",
    title: `下载 ${normalizedFileName}`,
  };
};
