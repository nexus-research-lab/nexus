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
        className="flex-1 max-lg:h-12 max-lg:rounded-[12px] max-lg:px-4"
        inputClassName="text-sm max-lg:text-base"
        onChange={onChange}
        placeholder={placeholder}
        value={value}
      />
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

/** 中文注释：搜索区尾部动作是 ghost icon button —— 默认无底无边框，hover 才出现暖灰底，消费者只提供业务图标与命令。 */
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
        "flex h-8 w-8 items-center justify-center rounded-[8px] text-(--icon-muted) transition-[background,color] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default) max-lg:h-12 max-lg:w-12 max-lg:rounded-[12px]",
        className,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}
