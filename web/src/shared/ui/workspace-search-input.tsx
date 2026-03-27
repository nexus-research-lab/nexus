"use client";

import { ChangeEvent } from "react";
import { Search } from "lucide-react";

import { cn } from "@/lib/utils";

interface WorkspaceSearchInputProps {
  value: string;
  placeholder?: string;
  class_name?: string;
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
      "home-glass-input inline-flex items-center gap-2 rounded-full px-4 py-2.5 text-sm text-slate-700/62",
      class_name,
    )}>
      <Search className="h-4 w-4" />
      <input
        className={cn(
          "min-w-0 flex-1 bg-transparent text-sm text-slate-950/86 outline-none placeholder:text-slate-500",
          input_class_name,
        )}
        onChange={handle_change}
        placeholder={placeholder}
        value={value}
      />
    </label>
  );
}
