"use client";

import { useEffect, useRef, useState } from "react";
import DOMPurify from "dompurify";

import { postProcessMermaidSvg } from "./mermaid-svg-postprocess";

const MERMAID_STREAM_RENDER_DELAY = 300;

type MermaidModule = typeof import("mermaid")["default"];

let mermaidModulePromise: Promise<MermaidModule> | null = null;

function loadMermaidModule(): Promise<MermaidModule> {
  mermaidModulePromise ??= import("mermaid").then((module) => module.default);
  return mermaidModulePromise;
}

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
  isStreaming: boolean,
  renderIdPrefix: string,
): MermaidRenderState {
  const normalizedChart = chart.trim();
  const latestChartRef = useRef(normalizedChart);
  const renderIndexRef = useRef(0);
  const renderStateKey = `${normalizedChart}\x1f${isStreaming ? "streaming" : "static"}`;
  const [activeRenderStateKey, setActiveRenderStateKey] = useState(renderStateKey);
  const [renderState, setRenderState] = useState<MermaidRenderState>({
    error: null,
    is_rendering: Boolean(normalizedChart),
    svg: "",
  });

  if (activeRenderStateKey !== renderStateKey) {
    setActiveRenderStateKey(renderStateKey);
    setRenderState((previous) => ({
      error: null,
      is_rendering: Boolean(normalizedChart),
      svg: isStreaming ? previous.svg : "",
    }));
  }

  useEffect(() => {
    latestChartRef.current = normalizedChart;
  }, [normalizedChart]);

  useEffect(() => {
    let cancelled = false;

    if (!normalizedChart) return;

    const commitRenderError = (message: string) => {
      if (cancelled || latestChartRef.current !== normalizedChart) {
        return;
      }

      setRenderState((previous) => ({
        error: isStreaming ? null : message,
        is_rendering: false,
        svg: isStreaming ? previous.svg : "",
      }));
    };

    const render = async () => {
      try {
        const mermaid = await loadMermaidModule();
        mermaid.initialize(MERMAID_CONFIG);
        const parseResult = await mermaid.parse(normalizedChart, { suppressErrors: true });
        if (!parseResult) {
          commitRenderError("Mermaid 源码语法无效");
          return;
        }

        const renderId = `${renderIdPrefix}-${renderIndexRef.current}`;
        renderIndexRef.current += 1;
        const result = await mermaid.render(renderId, normalizedChart);
        if (cancelled || latestChartRef.current !== normalizedChart) {
          return;
        }

        setRenderState({
          error: null,
          is_rendering: false,
          svg: DOMPurify.sanitize(postProcessMermaidSvg(result.svg), {
            USE_PROFILES: { svg: true, svgFilters: true },
          }),
        });
      } catch (renderError) {
        commitRenderError(renderError instanceof Error ? renderError.message : "Mermaid 渲染失败");
      }
    };

    // Mermaid 流式输入常处在半截语法状态，防抖后只提交仍然最新的合法 SVG。
    const timeoutId = setTimeout(
      () => void render(),
      isStreaming ? MERMAID_STREAM_RENDER_DELAY : 0,
    );
    return () => {
      cancelled = true;
      clearTimeout(timeoutId);
    };
  }, [isStreaming, normalizedChart, renderIdPrefix]);

  return renderState;
}
