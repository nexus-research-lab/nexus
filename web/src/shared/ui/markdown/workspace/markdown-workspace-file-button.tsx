/**
 * INPUT: 已解析的文件路径、标签与消费侧绑定作用域的打开命令。
 * OUTPUT: 可通过鼠标或键盘打开文件的通用 Markdown 按钮，提示跟随当前语言。
 * POS: 文件链接交互原语；不解释 Agent、Session 或文件资源权限。
 */
"use client";

import { UiTooltip } from "@/shared/ui/overlay/tooltip";
import { type ReactNode } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";

interface WorkspaceFileButtonProps {
  label: ReactNode;
  path: string;
  onOpenWorkspaceFile: (path: string) => void;
}

export function WorkspaceFileButton({
  label,
  path,
  onOpenWorkspaceFile,
}: WorkspaceFileButtonProps) {
  const { t } = useI18n();
  return (
    <UiTooltip label={t("markdown.workspace.open_file", { path })}><button
      className="content-workspace-file message-code-font max-w-full px-1.5 py-0.5 text-left align-baseline text-[0.86em] leading-[1.25]"
      onClick={() => onOpenWorkspaceFile(path)}

      type="button"
    >
      <span className="max-w-full whitespace-pre-wrap break-words">{label}</span>
    </button></UiTooltip>
  );
}
