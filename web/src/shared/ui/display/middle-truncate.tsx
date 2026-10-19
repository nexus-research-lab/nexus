// INPUT: 需要在有限宽度内展示的文件名或路径末段，以及外部布局 class。
// OUTPUT: 主体单行省略、扩展名始终可见的行内文本。
// POS: 长文件名截断的唯一展示原语；不解析路径归属，也不提供悬浮提示。

import { cn } from "@/shared/ui/class-name";

interface UiMiddleTruncateProps {
  className?: string;
  text: string;
}

// 扩展名最多保留 10 个字符；点号位于开头（.env）或没有扩展名时整体按普通省略处理。
const EXTENSION_PATTERN = /(\.[A-Za-z0-9_-]{1,10})$/;

function splitExtension(text: string): [string, string] {
  const match = EXTENSION_PATTERN.exec(text);
  if (!match || match.index === 0) {
    return [text, ""];
  }
  return [text.slice(0, match.index), match[1]];
}

/**
 * 普通尾部省略会把 `quarterly_report_final.xlsx` 截成 `quarterly_rep…`，丢掉文件类型；
 * 这里让主体收缩、扩展名固定，得到 `quarterly_rep….xlsx`。
 */
export function UiMiddleTruncate({ className, text }: UiMiddleTruncateProps) {
  const [stem, extension] = splitExtension(text);
  if (!extension) {
    return <span className={cn("block min-w-0 max-w-full truncate", className)}>{text}</span>;
  }
  // 完整名称只作为一个文本节点交给读屏和文本查询；拆开的可见片段是纯视觉投影。
  return (
    <span className={cn("flex min-w-0 max-w-full", className)}>
      <span className="sr-only">{text}</span>
      <span aria-hidden="true" className="min-w-0 truncate">{stem}</span>
      <span aria-hidden="true" className="shrink-0">{extension}</span>
    </span>
  );
}
