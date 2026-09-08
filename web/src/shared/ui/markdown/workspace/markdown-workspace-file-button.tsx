/**
 * INPUT: 已解析的文件路径、标签与消费侧绑定作用域的打开命令。
 * OUTPUT: 可通过鼠标或键盘打开文件的通用 Markdown 按钮。
 * POS: 文件链接交互原语；不解释 Agent、Session 或文件资源权限。
 */
"use client";

import { type ReactNode } from "react";

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
  return (
    <button
      className="content-workspace-file message-code-font max-w-full px-1.5 py-0.5 text-left align-baseline text-[0.86em] leading-[1.25]"
      onClick={() => onOpenWorkspaceFile(path)}
      title={`Open ${path}`}
      type="button"
    >
      <span className="max-w-full whitespace-pre-wrap break-words">{label}</span>
    </button>
  );
}
