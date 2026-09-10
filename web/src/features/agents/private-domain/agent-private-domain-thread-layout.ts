// INPUT: Agent 私域线程目录的 compact、联合状态与调用方外部布局 class。
// OUTPUT: 列表容器、行密度和共享 Typography 的稳定视图 recipe。
// POS: Agent 私域线程布局所有者；不读取线程数据、选中状态或本地化文案。

import { cn } from "@/shared/ui/class-name";
import type { UiListRowDensity } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import type { PrivateThreadListPresentation } from "./agent-private-domain-thread-model";

export interface PrivateThreadListLayout {
  containerClassName: string;
  density: UiListRowDensity;
  listClassName: string;
  rowClassName: string;
  summaryClassName: string;
  timestampClassName: string;
  titleClassName: string;
}

export function getPrivateThreadListLayout({
  className,
  compact,
  kind,
}: {
  className?: string;
  compact: boolean;
  kind: PrivateThreadListPresentation["kind"];
}): PrivateThreadListLayout {
  const density: UiListRowDensity = compact ? "dense" : "compact";
  const readyPadding = compact ? "p-1.5" : "p-2";
  const stateClassName = kind === "loading"
    ? "flex items-center justify-center"
    : kind === "empty"
      ? "flex flex-col items-center justify-center gap-2 px-4 text-center"
      : cn("soft-scrollbar min-h-0 overflow-y-auto", readyPadding);

  return {
    containerClassName: cn(stateClassName, className),
    density,
    listClassName: "space-y-0.5",
    rowClassName: compact ? "items-start gap-2" : "items-start gap-2.5",
    summaryClassName: cn(
      "mt-1 line-clamp-1 leading-4 [&_*]:leading-4",
      getUiTypographyClassName({ role: "metadata", tone: "muted" }),
    ),
    timestampClassName: cn(
      "ml-auto shrink-0 tabular-nums",
      getUiTypographyClassName({ role: "caption", tone: "soft" }),
    ),
    titleClassName: cn(
      "min-w-0 flex-1 truncate",
      getUiTypographyClassName({
        role: "metadata",
        tone: "strong",
        weight: "semibold",
      }),
    ),
  };
}
