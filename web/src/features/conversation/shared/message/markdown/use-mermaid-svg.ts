"use client";

import { useEffect, useRef, useState } from "react";
import DOMPurify from "dompurify";
import mermaid from "mermaid";

const MERMAID_STREAM_RENDER_DELAY = 300;

const MERMAID_CONFIG = {
  htmlLabels: false,
  startOnLoad: false,
  securityLevel: "strict" as const,
  theme: "default" as const,
};

export interface MermaidRenderState {
  error: string | null;
  is_rendering: boolean;
  svg: string;
}

export function useMermaidSvg(
  chart: string,
  is_streaming: boolean,
  render_id_prefix: string,
): MermaidRenderState {
  const normalized_chart = chart.trim();
  const latest_chart_ref = useRef(normalized_chart);
  const render_index_ref = useRef(0);
  const [render_state, set_render_state] = useState<MermaidRenderState>({
    error: null,
    is_rendering: false,
    svg: "",
  });

  useEffect(() => {
    latest_chart_ref.current = normalized_chart;
  }, [normalized_chart]);

  useEffect(() => {
    let cancelled = false;

    if (!normalized_chart) {
      set_render_state({
        error: null,
        is_rendering: false,
        svg: "",
      });
      return;
    }

    set_render_state((previous) => ({
      error: null,
      is_rendering: true,
      svg: is_streaming ? previous.svg : "",
    }));

    const commit_render_error = (message: string) => {
      if (cancelled || latest_chart_ref.current !== normalized_chart) {
        return;
      }

      set_render_state((previous) => ({
        error: is_streaming ? null : message,
        is_rendering: false,
        svg: is_streaming ? previous.svg : "",
      }));
    };

    const render = async () => {
      try {
        mermaid.initialize(MERMAID_CONFIG);
        const parse_result = await mermaid.parse(normalized_chart, { suppressErrors: true });
        if (!parse_result) {
          commit_render_error("Mermaid 源码语法无效");
          return;
        }

        const render_id = `${render_id_prefix}-${render_index_ref.current}`;
        render_index_ref.current += 1;
        const result = await mermaid.render(render_id, normalized_chart);
        if (cancelled || latest_chart_ref.current !== normalized_chart) {
          return;
        }

        set_render_state({
          error: null,
          is_rendering: false,
          svg: DOMPurify.sanitize(result.svg, {
            USE_PROFILES: { svg: true, svgFilters: true },
          }),
        });
      } catch (render_error) {
        commit_render_error(render_error instanceof Error ? render_error.message : "Mermaid 渲染失败");
      }
    };

    // Mermaid 流式输入常处在半截语法状态，防抖后只提交仍然最新的合法 SVG。
    const timeout_id = setTimeout(
      () => void render(),
      is_streaming ? MERMAID_STREAM_RENDER_DELAY : 0,
    );
    return () => {
      cancelled = true;
      clearTimeout(timeout_id);
    };
  }, [is_streaming, normalized_chart, render_id_prefix]);

  return render_state;
}
