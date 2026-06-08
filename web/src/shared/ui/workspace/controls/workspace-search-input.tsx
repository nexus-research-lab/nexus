"use client";

import { type ReactNode } from "react";

import { UiSearchInput } from "@/shared/ui/form-control";

interface WorkspaceSearchInputProps {
  value: string;
  placeholder?: string;
  action?: ReactNode;
  /** 中文注释：这里只保留布局层入口，比如占满宽度或响应式显隐，不覆写控件质感。 */
  class_name?: string;
  /** 中文注释：这里只保留输入宽度这类槽位调整，不覆写输入本体颜色和边框。 */
  input_class_name?: string;
  on_change: (value: string) => void;
}

export function WorkspaceSearchInput({
  value,
  placeholder = "搜索",
  action,
  class_name,
  input_class_name,
  on_change,
}: WorkspaceSearchInputProps) {
  return (
    <UiSearchInput
      action={action}
      class_name={class_name}
      input_class_name={input_class_name}
      on_change={on_change}
      placeholder={placeholder}
      value={value}
    />
  );
}
