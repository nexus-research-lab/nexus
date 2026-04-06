"use client";

import { ChangeEvent } from "react";
import { Search } from "lucide-react";

import { cn } from "@/lib/utils";

const SEARCH_INPUT_SHELL_CLASS_NAME =
  "input-shell inline-flex items-center gap-2 rounded-full px-4 py-2.5 text-sm text-slate-700/86 transition duration-150 ease-out hover:border-[var(--surface-interactive-hover-border)] hover:bg-[var(--surface-interactive-hover-background)] focus-within:border-[var(--surface-interactive-active-border)] focus-within:bg-[var(--surface-interactive-active-background)]";
const SEARCH_INPUT_FIELD_CLASS_NAME =
  "min-w-0 flex-1 bg-transparent text-sm text-slate-950/98 outline-none placeholder:text-slate-500/80";

interface WorkspaceSearchInputProps {
  value: string;
  placeholder?: string;
  /** 中文注释：这里只保留布局层入口，比如占满宽度或响应式显隐，不覆写控件质感。 */
  class_name?: string;
  /** 中文注释：这里只保留输入宽度这类槽位调整，不覆写输入本体颜色和边框。 */
  input_class_name?: string;
  on_change: (value: string) => void;
}

export function WorkspaceSearchInput({
  value,
  placeholder = "搜索",
  class_name,
  input_class_name,
  on_change,
}: WorkspaceSearchInputProps) {
  const handle_change = (event: ChangeEvent<HTMLInputElement>) => {
    on_change(event.target.value);
  };

  return (
    <label className={cn(
      SEARCH_INPUT_SHELL_CLASS_NAME,
      class_name,
    )}>
      <Search className="h-4 w-4 text-slate-600/84" />
      <input
        className={cn(
          SEARCH_INPUT_FIELD_CLASS_NAME,
          input_class_name,
        )}
        onChange={handle_change}
        placeholder={placeholder}
        value={value}
      />
    </label>
  );
}
