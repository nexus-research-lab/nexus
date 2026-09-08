// INPUT: Mermaid 图表参数和延迟加载中的模块边界。
// OUTPUT: 与正式图表同尺寸、使用共享 Spinner 和排版的 Suspense 占位。
// POS: Mermaid 延迟加载适配层；不拥有渲染状态机或 Spinner recipe。

"use client";

import { LoaderCircle } from "lucide-react";
import { lazy, Suspense } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { type MermaidViewProps } from "./mermaid-view";

const LazyMermaidViewInner = lazy(async () => {
  const module = await import("./mermaid-view");
  return { default: module.MermaidView };
});

function MermaidViewLoadingFallback({
  className: className,
  compact = false,
  constrainHeight: constrainHeight = true,
}: Pick<MermaidViewProps, "className" | "compact" | "constrainHeight">) {
  const { t } = useI18n();

  return (
    <div
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background)",
        compact ? "my-2 max-h-[360px]" : constrainHeight ? "my-3 max-h-[460px]" : "min-h-0",
        className,
      )}
    >
      <div
        aria-busy="true"
        aria-live="polite"
        className={cn(
          "flex items-center justify-center px-4",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          compact ? "min-h-24 py-6" : constrainHeight ? "min-h-56 py-8" : "min-h-[240px] flex-1 py-8",
        )}
        role="status"
      >
        <LoaderCircle
          aria-hidden
          className={getUiSpinnerClassName({ size: "md" }, "mr-2")}
        />
        {t("markdown.mermaid.loading")}
      </div>
    </div>
  );
}

export function LazyMermaidView(props: MermaidViewProps) {
  return (
    <Suspense
      fallback={(
        <MermaidViewLoadingFallback
          className={props.className}
          compact={props.compact}
          constrainHeight={props.constrainHeight}
        />
      )}
    >
      <LazyMermaidViewInner {...props} />
    </Suspense>
  );
}
