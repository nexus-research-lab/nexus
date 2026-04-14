"use client";

import { ChangeEvent } from "react";
import { Search } from "lucide-react";

import { cn } from "@/lib/utils";

const SEARCH_INPUT_SHELL_CLASS_NAME =
  "input-shell inline-flex items-center gap-2 rounded-full px-4 py-2.5 text-sm text-(--text-default) transition duration-150 ease-out hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)";
const SEARCH_INPUT_FIELD_CLASS_NAME =
  "min-w-0 flex-1 bg-transparent text-sm text-(--text-strong) outline-none shadow-none ring-0 placeholder:text-(--text-soft) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none";

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
      <Search className="h-4 w-4 text-(--icon-default)" />
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
