// INPUT: 侧栏搜索值、完整搜索名称、变更命令与可选尾部动作。
// OUTPUT: 简短可见提示、完整可访问名称的响应式搜索行及独立共享 IconButton。
// POS: 侧栏搜索组合 pattern；只拥有响应式布局，不筛选资源或派发远端请求。

import type { ButtonHTMLAttributes, ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";

import { UiSearchInput } from "./form-control";

/** 中文注释：侧栏搜索只负责统一输入壳层，业务动作仍由消费者传入。 */
export function SidebarSearchField({
  action,
  label,
  onChange,
  value,
}: {
  action?: ReactNode;
  label: string;
  onChange: (value: string) => void;
  value: string;
}) {
  const { t } = useI18n();

  // 右侧 8px 让 36px 尾部动作的中轴落在侧栏右缘内 26px，与品牌栏折叠按钮、底栏设置按钮同一垂线；
  // 位于主侧栏目录时从右侧 8px 中扣掉滚动条预留宽度（macOS 为 0，Windows/Web 为经典细滚动条宽）；
  // 只能收 padding 不能用负 margin，越过 padding box 会触发横向滚动或被裁切。
  return (
    <div className="flex items-center gap-2 pl-2.5 pr-[max(0px,calc(0.5rem-var(--sidebar-directory-gutter,0px)))] pb-1.5 max-[559px]:gap-3 max-[559px]:px-4 max-[559px]:pb-3">
      <UiSearchInput
        aria-label={label}
        className="flex-1 max-[559px]:h-12 max-[559px]:rounded-(--radius-control-lg) max-[559px]:px-4"
        onChange={onChange}
        placeholder={t("common.search")}
        value={value}
      />
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

/** 中文注释：组合层只让尾部动作与响应式字段同高；状态、提示与按钮语义归 IconButton。 */
export function SidebarSearchAction({
  children,
  className,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
}) {
  return (
    <UiIconButton
      className={cn(
        "rounded-(--radius-control-md) [&>svg]:h-4 [&>svg]:w-4 max-[559px]:h-12 max-[559px]:w-12 max-[559px]:rounded-(--radius-control-lg)",
        className,
      )}
      size="lg"
      type={type}
      {...props}
    >
      {children}
    </UiIconButton>
  );
}
