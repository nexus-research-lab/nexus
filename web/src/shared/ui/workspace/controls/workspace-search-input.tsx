"use client";

import { type ReactNode } from "react";

import { UiSearchInput } from "@/shared/ui/form/form-control";

interface WorkspaceSearchInputProps {
  value: string;
  placeholder?: string;
  action?: ReactNode;
  /** 中文注释：这里只保留布局层入口，比如占满宽度或响应式显隐，不覆写控件质感。 */
  className?: string;
  /** 中文注释：这里只保留输入宽度这类槽位调整，不覆写输入本体颜色和边框。 */
  inputClassName?: string;
  onChange: (value: string) => void;
}

export function WorkspaceSearchInput({
  value,
  placeholder = "搜索",
  action,
  className: className,
  inputClassName: inputClassName,
  onChange: onChange,
}: WorkspaceSearchInputProps) {
  return (
    <UiSearchInput
      action={action}
      className={className}
      inputClassName={inputClassName}
      onChange={onChange}
      placeholder={placeholder}
      value={value}
    />
  );
}
