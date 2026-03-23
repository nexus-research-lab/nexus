"use client";

import { type PointerEvent as ReactPointerEvent, useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";

import { type BlobPoint } from "@/components/home/hero-blob-shape";
import { type BlobDebugTarget } from "@/components/home/hero-blob-debug-hooks";

function useDebugSvgRect(
  debugEnabled: boolean,
  svgRef: React.RefObject<SVGSVGElement | null>,
): DOMRect | null {
  const [svgRect, setSvgRect] = useState<DOMRect | null>(null);

  useEffect(() => {
    if (!debugEnabled || !svgRef.current) {
      setSvgRect(null);
      return;
    }

    const svgElement = svgRef.current;
    const updateRect = () => {
      setSvgRect(svgElement.getBoundingClientRect());
    };

    updateRect();
    const resizeObserver = new ResizeObserver(updateRect);
    resizeObserver.observe(svgElement);
    window.addEventListener("resize", updateRect);
    window.addEventListener("scroll", updateRect, true);

    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", updateRect);
      window.removeEventListener("scroll", updateRect, true);
    };
  }, [debugEnabled, svgRef]);

  return svgRect;
}

function DebugPortalHandles({
  debugEnabled,
  handleClassName,
  onPointPointerDown,
  onPointPointerUp,
  points,
  svgRect,
  viewBoxHeight,
  viewBoxWidth,
}: {
  debugEnabled: boolean;
  handleClassName?: string;
  onPointPointerDown: (index: number) => (event: ReactPointerEvent<Element>) => void;
  onPointPointerUp: (event: ReactPointerEvent<Element>) => void;
  points: BlobPoint[];
  svgRect: DOMRect | null;
  viewBoxHeight: number;
  viewBoxWidth: number;
}) {
  if (!debugEnabled || !svgRect) {
    return null;
  }

  return createPortal(
    <>
      {points.map((point, index) => (
        <button
          key={`point-handle-${index}`}
          className={cn(
            "fixed z-[200] h-8 w-8 -translate-x-1/2 -translate-y-1/2 rounded-full border border-transparent bg-transparent",
            handleClassName,
          )}
          onPointerDown={onPointPointerDown(index)}
          onPointerUp={onPointPointerUp}
          style={{
            cursor: "grab",
            left: svgRect.left + (point.x / viewBoxWidth) * svgRect.width,
            top: svgRect.top + (point.y / viewBoxHeight) * svgRect.height,
          }}
          type="button"
        />
      ))}
    </>,
    document.body,
  );
}

export function BlobDebugPanel({
  countLabel = "当前点数",
  currentTarget,
  description = "直接拖点调轮廓，双击轮廓线新增点。",
  onCopy,
  onReset,
  panelClassName,
  points,
  setTarget,
  target,
  title,
}: {
  countLabel?: string;
  currentTarget: BlobDebugTarget;
  description?: string;
  onCopy: () => void;
  onReset: () => void;
  panelClassName: string;
  points: BlobPoint[];
  setTarget: (target: BlobDebugTarget) => void;
  target: BlobDebugTarget;
  title: string;
}) {
  const targetLabels: Record<BlobDebugTarget, string> = {
    hero: "Hero",
    input: "Input",
  };

  return (
    <div
      className={cn(
        "pointer-events-auto fixed z-[60] w-[300px] rounded-2xl border border-white/18 bg-black/55 p-4 text-white shadow-[0_18px_50px_rgba(0,0,0,0.35)] backdrop-blur-xl",
        panelClassName,
      )}
    >
      <div className="mb-3">
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-white/74">{title}</p>
        <p className="mt-1 text-[11px] leading-5 text-white/46">{description}</p>
      </div>

      <div className="rounded-xl border border-white/10 bg-white/4 px-3 py-2 text-[11px] text-white/42">
        {countLabel}：{points.length}
      </div>

      <div className="mt-3 flex items-center gap-2">
        <button
          className={cn(
            "rounded-full border px-3 py-1.5 text-[11px] transition-colors",
            target === "hero"
              ? "border-white/28 bg-white/12 text-white"
              : "border-white/14 text-white/62 hover:text-white",
          )}
          onClick={() => setTarget("hero")}
          type="button"
        >
          编辑 Hero
        </button>
        <button
          className={cn(
            "rounded-full border px-3 py-1.5 text-[11px] transition-colors",
            target === "input"
              ? "border-white/28 bg-white/12 text-white"
              : "border-white/14 text-white/62 hover:text-white",
          )}
          onClick={() => setTarget("input")}
          type="button"
        >
          编辑 Input
        </button>
      </div>

      <div className="mt-3 flex items-center gap-2">
        <button
          className="rounded-full border border-white/14 px-3 py-1.5 text-[11px] text-white/62 transition-colors hover:text-white"
          onClick={onCopy}
          type="button"
        >
          复制点位 JSON
        </button>
        <button
          className="rounded-full border border-white/14 px-3 py-1.5 text-[11px] text-white/62 transition-colors hover:text-white"
          onClick={onReset}
          type="button"
        >
          重置
        </button>
      </div>

      <p className="mt-3 text-[11px] leading-5 text-white/42">
        当前激活层：{targetLabels[currentTarget]}
      </p>
    </div>
  );
}

export function BlobDebugOverlay({
  color,
  debugEnabled,
  fill,
  onPathDoubleClick,
  onPointPointerDown,
  onPointPointerUp,
  path,
  points,
  stroke,
  strokeWidth,
  svgRef,
  viewBoxHeight,
  viewBoxWidth,
}: {
  color: string;
  debugEnabled: boolean;
  fill: string;
  onPathDoubleClick: (event: ReactPointerEvent<SVGPathElement>) => void;
  onPointPointerDown: (index: number) => (event: ReactPointerEvent<Element>) => void;
  onPointPointerUp: (event: ReactPointerEvent<Element>) => void;
  path: string;
  points: BlobPoint[];
  stroke: string;
  strokeWidth: number;
  svgRef: React.RefObject<SVGSVGElement | null>;
  viewBoxHeight: number;
  viewBoxWidth: number;
}) {
  const debugAreaFill = debugEnabled ? color.replace(/0?\.\d+\)$/, "0.12)") : fill;
  const debugStroke = debugEnabled ? color.replace(/0?\.\d+\)$/, "0.78)") : stroke;
  const svgRect = useDebugSvgRect(debugEnabled, svgRef);

  return (
    <>
      <svg
        ref={svgRef}
        aria-hidden="true"
        className={cn("absolute inset-0 h-full w-full", debugEnabled ? "pointer-events-auto z-20" : "pointer-events-none")}
        preserveAspectRatio="none"
        viewBox={`0 0 ${viewBoxWidth} ${viewBoxHeight}`}
      >
        <path
          d={path}
          fill={debugAreaFill}
          pointerEvents="none"
          stroke={debugStroke}
          strokeWidth={debugEnabled ? Math.max(strokeWidth, 2.5) : strokeWidth}
        />
        <path
          d={path}
          fill="none"
          onDoubleClick={onPathDoubleClick}
          pointerEvents={debugEnabled ? "stroke" : "none"}
          stroke={debugEnabled ? "rgba(255,255,255,0.14)" : "rgba(255,255,255,0.001)"}
          strokeWidth={Math.max(strokeWidth, 18)}
        />

        {debugEnabled && (
          <g>
            <path
              d={path}
              fill="none"
              pointerEvents="none"
              stroke="rgba(255,255,255,0.72)"
              strokeDasharray="8 6"
              strokeWidth="1.3"
            />
            {points.map((point, index) => (
              <g key={`point-visual-${index}`}>
                <circle
                  cx={point.x}
                  cy={point.y}
                  fill="rgba(255,255,255,0.92)"
                  pointerEvents="none"
                  r="8"
                  stroke={color}
                  strokeWidth="3"
                />
                <text
                  fill="rgba(255,255,255,0.72)"
                  fontSize="12"
                  pointerEvents="none"
                  textAnchor="middle"
                  x={point.x}
                  y={point.y - 16}
                >
                  {index + 1}
                </text>
              </g>
            ))}
          </g>
        )}
      </svg>

      <DebugPortalHandles
        debugEnabled={debugEnabled}
        onPointPointerDown={onPointPointerDown}
        onPointPointerUp={onPointPointerUp}
        points={points}
        svgRect={svgRect}
        viewBoxHeight={viewBoxHeight}
        viewBoxWidth={viewBoxWidth}
      />
    </>
  );
}

export function BlobDebugController({
  active,
  color,
  currentTarget,
  enabled,
  fill,
  onCopy,
  onPathDoubleClick,
  onPointPointerDown,
  onPointPointerUp,
  onReset,
  panelClassName,
  path,
  points,
  setTarget,
  showPanel = true,
  stroke,
  strokeWidth,
  svgRef,
  target,
  title,
  viewBoxHeight,
  viewBoxWidth,
}: {
  active: boolean;
  color: string;
  currentTarget: BlobDebugTarget;
  enabled: boolean;
  fill: string;
  onCopy: () => void;
  onPathDoubleClick: (event: ReactPointerEvent<SVGPathElement>) => void;
  onPointPointerDown: (index: number) => (event: ReactPointerEvent<Element>) => void;
  onPointPointerUp: (event: ReactPointerEvent<Element>) => void;
  onReset: () => void;
  panelClassName: string;
  path: string;
  points: BlobPoint[];
  setTarget: (target: BlobDebugTarget) => void;
  showPanel?: boolean;
  stroke: string;
  strokeWidth: number;
  svgRef: React.RefObject<SVGSVGElement | null>;
  target: BlobDebugTarget;
  title: string;
  viewBoxHeight: number;
  viewBoxWidth: number;
}) {
  return (
    <>
      <BlobDebugOverlay
        color={color}
        debugEnabled={enabled && active}
        fill={fill}
        onPathDoubleClick={onPathDoubleClick}
        onPointPointerDown={onPointPointerDown}
        onPointPointerUp={onPointPointerUp}
        path={path}
        points={points}
        stroke={stroke}
        strokeWidth={strokeWidth}
        svgRef={svgRef}
        viewBoxHeight={viewBoxHeight}
        viewBoxWidth={viewBoxWidth}
      />

      {enabled && showPanel && (
        <BlobDebugPanel
          currentTarget={currentTarget}
          onCopy={onCopy}
          onReset={onReset}
          panelClassName={panelClassName}
          points={points}
          setTarget={setTarget}
          target={target}
          title={title}
        />
      )}
    </>
  );
}
