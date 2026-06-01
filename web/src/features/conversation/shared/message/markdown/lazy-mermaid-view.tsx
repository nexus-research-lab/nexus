"use client";

import { lazy, Suspense } from "react";
import { LoaderCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { type MermaidViewProps } from "./mermaid-view";

const LazyMermaidViewInner = lazy(async () => {
  const module = await import("./mermaid-view");
  return { default: module.MermaidView };
});

function MermaidViewLoadingFallback({
  class_name,
  compact = false,
  constrain_height = true,
}: Pick<MermaidViewProps, "class_name" | "compact" | "constrain_height">) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background)",
        compact ? "my-2 max-h-[360px]" : constrain_height ? "my-3 max-h-[460px]" : "min-h-0",
        class_name,
      )}
    >
      <div
        className={cn(
          "flex items-center justify-center px-4 text-sm text-(--text-muted)",
          compact ? "min-h-24 py-6" : constrain_height ? "min-h-56 py-8" : "min-h-[240px] flex-1 py-8",
        )}
      >
        <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
        正在加载 Mermaid 预览
      </div>
    </div>
  );
}

export function LazyMermaidView(props: MermaidViewProps) {
  return (
    <Suspense
      fallback={(
        <MermaidViewLoadingFallback
          class_name={props.class_name}
          compact={props.compact}
          constrain_height={props.constrain_height}
        />
      )}
    >
      <LazyMermaidViewInner {...props} />
    </Suspense>
  );
}
