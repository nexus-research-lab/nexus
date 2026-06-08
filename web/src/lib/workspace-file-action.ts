import { is_desktop_runtime } from "@/config/desktop-runtime";

export interface WorkspaceFileExternalActionCopy {
  aria_label: string;
  label: string;
  mode: "download" | "reveal";
  title: string;
}

export const get_workspace_file_external_action_copy = (
  file_name?: string,
): WorkspaceFileExternalActionCopy => {
  const normalized_file_name = file_name?.trim() || "文件";
  if (is_desktop_runtime()) {
    return {
      aria_label: `在文件夹中显示 ${normalized_file_name}`,
      label: "打开",
      mode: "reveal",
      title: `在文件夹中显示 ${normalized_file_name}`,
    };
  }
  return {
    aria_label: `下载 ${normalized_file_name}`,
    label: "下载",
    mode: "download",
    title: `下载 ${normalized_file_name}`,
  };
};
