// INPUT: 目录内容/筛选选项、当前值与切换命令。
// OUTPUT: 适合目录工具栏的紧凑、自适应宽度下划线标签组。
// POS: 跨领域目录标签 pattern；不认识 Capability、业务筛选状态或内容面板。

import {
  UiTabs,
  type UiTabsProps,
} from "@/shared/ui/navigation/tabs";

type UiDirectoryTabsProps<TValue extends string> = Pick<
  UiTabsProps<TValue>,
  "activeValue" | "ariaLabel" | "navAnchor" | "onChange" | "options"
>;

export function UiDirectoryTabs<TValue extends string>({
  activeValue,
  ariaLabel,
  navAnchor,
  onChange,
  options,
}: UiDirectoryTabsProps<TValue>) {
  return (
    <UiTabs
      activeValue={activeValue}
      ariaLabel={ariaLabel}
      className="h-8 w-fit max-w-full shrink-0 self-start"
      density="compact"
      itemClassName="h-8 px-3"
      navAnchor={navAnchor}
      onChange={onChange}
      options={options}
    />
  );
}
