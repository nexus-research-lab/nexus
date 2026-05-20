"use client";

import { useEffect, useId, useState } from "react";
import DOMPurify from "dompurify";
import mermaid from "mermaid";
import { AlertTriangle, LoaderCircle } from "lucide-react";

import { cn } from "@/lib/utils";

interface MermaidViewProps {
  chart: string;
  compact?: boolean;
  class_name?: string;
}

export function MermaidView({ chart, compact = false, class_name }: MermaidViewProps) {
  const render_id = `mermaid-${useId().replace(/:/g, "")}`;
  const [svg, set_svg] = useState("");
  const [error, set_error] = useState<string | null>(null);
  const [is_rendering, set_is_rendering] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const render = async () => {
      set_is_rendering(true);
      set_error(null);
      set_svg("");
      try {
        mermaid.initialize({
          startOnLoad: false,
          securityLevel: "strict",
          theme: "default",
        });
        const result = await mermaid.render(render_id, chart);
        if (cancelled) {
          return;
        }
        set_svg(DOMPurify.sanitize(result.svg, {
          USE_PROFILES: { svg: true, svgFilters: true },
        }));
      } catch (render_error) {
        if (!cancelled) {
          set_error(render_error instanceof Error ? render_error.message : "Mermaid 渲染失败");
        }
      } finally {
        if (!cancelled) {
          set_is_rendering(false);
        }
      }
    };

    void render();
    return () => {
      cancelled = true;
    };
  }, [chart, render_id]);

  if (is_rendering) {
    return (
      <div className={cn("flex items-center justify-center text-(--text-muted)", compact ? "min-h-24" : "min-h-56", class_name)}>
        <LoaderCircle className="mr-2 h-4 w-4 animate-spin" />
        正在渲染图表
      </div>
    );
  }

  if (error) {
    return (
      <div className={cn("rounded-[8px] border border-destructive/20 bg-destructive/6 px-3 py-2 text-sm text-destructive", class_name)}>
        <div className="flex items-center gap-2 font-medium">
          <AlertTriangle className="h-4 w-4" />
          Mermaid 渲染失败
        </div>
        <pre className="mt-2 whitespace-pre-wrap break-words text-xs leading-5">{error}</pre>
      </div>
    );
  }

  return (
    <div
      className={cn(
        "mermaid-view flex min-w-0 justify-center overflow-auto rounded-[8px] bg-white p-4",
        compact ? "my-2 max-h-[420px]" : "h-full",
        class_name,
      )}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
