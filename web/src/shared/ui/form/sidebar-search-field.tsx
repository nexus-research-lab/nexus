import type { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

import { UiSearchInput } from "./form-control";

/** 中文注释：侧栏搜索只负责统一输入壳层，业务动作仍由消费者传入。 */
export function SidebarSearchField({
  action,
  onChange,
  placeholder,
  value,
}: {
  action?: ReactNode;
  onChange: (value: string) => void;
  placeholder: string;
  value: string;
}) {
  return (
    <div className="flex items-center gap-2 px-2.5 pb-1.5 max-lg:gap-3 max-lg:px-4 max-lg:pb-3">
      <UiSearchInput
        className="workbench-input-shell flex-1 border-(--surface-control-border) bg-(--surface-control-field-background) shadow-(--surface-control-field-shadow) hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) max-lg:h-12 max-lg:rounded-[12px] max-lg:px-4"
        inputClassName="text-[13px] max-lg:text-[15px]"
        onChange={onChange}
        placeholder={placeholder}
        value={value}
      />
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

/** 中文注释：搜索区尾部动作共享暖色轻抬升基座，消费者只提供业务图标与命令。 */
export function SidebarSearchAction({
  children,
  className,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
}) {
  return (
    <button
      className={cn(
        "flex h-8 w-8 items-center justify-center rounded-[8px] border border-(--surface-control-border) bg-(--surface-control-background) text-(--icon-muted) shadow-(--surface-control-shadow) transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--icon-default) max-lg:h-12 max-lg:w-12 max-lg:rounded-[12px]",
        className,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}
