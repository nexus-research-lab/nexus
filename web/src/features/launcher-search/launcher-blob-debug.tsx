"use client";

import { type PointerEvent as ReactPointerEvent, type RefObject, useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { cn } from "@/lib/utils";
import {
  BlobDebugControllerProps,
  BlobDebugOverlayProps,
  BlobDebugPanelProps,
} from "@/types/launcher-ui";

import { type BlobPoint } from "@/features/launcher-search/launcher-blob-shape";
import { type BlobDebugTarget } from "@/features/launcher-search/launcher-blob-debug-hooks";

function useDebugSvgRect(
  debug_enabled: boolean,
  svg_ref: RefObject<SVGSVGElement | null>,
): DOMRect | null {
  const [svgRect, setSvgRect] = useState<DOMRect | null>(null);

  useEffect(() => {
    if (!debug_enabled || !svg_ref.current) {
      setSvgRect(null);
      return;
    }

    const svgElement = svg_ref.current;
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
  }, [debug_enabled, svg_ref]);

  return svgRect;
}

function DebugPortalHandles({
  debug_enabled,
  handle_class_name,
  on_point_pointer_down,
  on_point_pointer_up,
  points,
  svgRect,
  view_box_height,
  view_box_width,
}: {
  debug_enabled: boolean;
  handle_class_name?: string;
  on_point_pointer_down: (index: number) => (event: ReactPointerEvent<Element>) => void;
  on_point_pointer_up: (event: ReactPointerEvent<Element>) => void;
  points: BlobPoint[];
  svgRect: DOMRect | null;
  view_box_height: number;
  view_box_width: number;
}) {
  if (!debug_enabled || !svgRect) {
    return null;
  }

  return createPortal(
    <>
      {points.map((point, index) => (
        <button
          key={`point-handle-${index}`}
          className={cn(
            "fixed z-[200] h-8 w-8 -translate-x-1/2 -translate-y-1/2 rounded-full border border-transparent bg-transparent",
            handle_class_name,
          )}
          onPointerDown={on_point_pointer_down(index)}
          onPointerUp={on_point_pointer_up}
          style={{
            cursor: "grab",
            left: svgRect.left + (point.x / view_box_width) * svgRect.width,
            top: svgRect.top + (point.y / view_box_height) * svgRect.height,
          }}
          type="button"
        />
      ))}
    </>,
    document.body,
  );
}

export function BlobDebugPanel({
  count_label = "当前点数",
  current_target,
  description = "直接拖点调轮廓，双击轮廓线新增点。",
  on_copy,
  on_reset,
  panel_class_name,
  points,
  set_target,
  target,
  title,
}: BlobDebugPanelProps) {
  const targetLabels: Record<BlobDebugTarget, string> = {
    hero: "Hero",
    input: "Input",
    panel: "Panel",
  };

  return (
    <div
      className={cn(
        "pointer-events-auto fixed z-[60] w-[300px] rounded-2xl border border-white/18 bg-black/55 p-4 text-white shadow-[0_18px_50px_rgba(0,0,0,0.35)] backdrop-blur-xl",
        panel_class_name,
      )}
    >
      <div className="mb-3">
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-white/74">{title}</p>
        <p className="mt-1 text-[11px] leading-5 text-white/46">{description}</p>
      </div>

      <div className="rounded-xl border border-white/10 bg-white/4 px-3 py-2 text-[11px] text-white/42">
        {count_label}：{points.length}
      </div>

      <div className="mt-3 flex items-center gap-2">
        <button
          className={cn(
            "rounded-full border px-3 py-1.5 text-[11px] transition-colors",
            target === "hero"
              ? "border-white/28 bg-white/12 text-white"
              : "border-white/14 text-white/62 hover:text-white",
          )}
          onClick={() => set_target("hero")}
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
          onClick={() => set_target("input")}
          type="button"
        >
          编辑 Input
        </button>
        <button
          className={cn(
            "rounded-full border px-3 py-1.5 text-[11px] transition-colors",
            target === "panel"
              ? "border-white/28 bg-white/12 text-white"
              : "border-white/14 text-white/62 hover:text-white",
          )}
          onClick={() => set_target("panel")}
          type="button"
        >
          编辑 Panel
        </button>
      </div>

      <div className="mt-3 flex items-center gap-2">
        <button
          className="rounded-full border border-white/14 px-3 py-1.5 text-[11px] text-white/62 transition-colors hover:text-white"
          onClick={on_copy}
          type="button"
        >
          复制点位 JSON
        </button>
        <button
          className="rounded-full border border-white/14 px-3 py-1.5 text-[11px] text-white/62 transition-colors hover:text-white"
          onClick={on_reset}
          type="button"
        >
          重置
        </button>
      </div>

      <p className="mt-3 text-[11px] leading-5 text-white/42">
        当前激活层：{targetLabels[current_target]}
      </p>
    </div>
  );
}

export function BlobDebugOverlay({
  color,
  debug_enabled,
  fill,
  on_path_double_click,
  on_point_pointer_down,
  on_point_pointer_up,
  path,
  points,
  stroke,
  stroke_width,
  svg_ref,
  view_box_height,
  view_box_width,
}: BlobDebugOverlayProps) {
  const debugAreaFill = debug_enabled ? color.replace(/0?\.\d+\)$/, "0.12)") : fill;
  const debugStroke = debug_enabled ? color.replace(/0?\.\d+\)$/, "0.78)") : stroke;
  const svgRect = useDebugSvgRect(debug_enabled, svg_ref);

  return (
    <>
      <svg
        ref={svg_ref}
        aria-hidden="true"
        className={cn("absolute inset-0 h-full w-full", debug_enabled ? "pointer-events-auto z-20" : "pointer-events-none")}
        preserveAspectRatio="none"
        viewBox={`0 0 ${view_box_width} ${view_box_height}`}
      >
        <path
          d={path}
          fill={debugAreaFill}
          pointerEvents="none"
          stroke={debugStroke}
          strokeWidth={debug_enabled ? Math.max(stroke_width, 2.5) : stroke_width}
        />
        <path
          d={path}
          fill="none"
          onDoubleClick={on_path_double_click}
          pointerEvents={debug_enabled ? "stroke" : "none"}
          stroke={debug_enabled ? "rgba(255,255,255,0.14)" : "rgba(255,255,255,0.001)"}
          strokeWidth={Math.max(stroke_width, 18)}
        />

        {debug_enabled && (
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
        debug_enabled={debug_enabled}
        on_point_pointer_down={on_point_pointer_down}
        on_point_pointer_up={on_point_pointer_up}
        points={points}
        svgRect={svgRect}
        view_box_height={view_box_height}
        view_box_width={view_box_width}
      />
    </>
  );
}

export function BlobDebugController({
  is_active,
  color,
  current_target,
  debug_enabled,
  fill,
  on_copy,
  on_path_double_click,
  on_point_pointer_down,
  on_point_pointer_up,
  on_reset,
  panel_class_name,
  path,
  points,
  set_target,
  show_panel = true,
  stroke,
  stroke_width,
  svg_ref,
  target,
  title,
  view_box_height,
  view_box_width,
}: BlobDebugControllerProps) {
  return (
    <>
      <BlobDebugOverlay
        color={color}
        debug_enabled={debug_enabled && is_active}
        fill={fill}
        on_path_double_click={on_path_double_click}
        on_point_pointer_down={on_point_pointer_down}
        on_point_pointer_up={on_point_pointer_up}
        path={path}
        points={points}
        stroke={stroke}
        stroke_width={stroke_width}
        svg_ref={svg_ref}
        view_box_height={view_box_height}
        view_box_width={view_box_width}
      />

      {debug_enabled && show_panel && (
        <BlobDebugPanel
          current_target={current_target}
          on_copy={on_copy}
          on_reset={on_reset}
          panel_class_name={panel_class_name}
          points={points}
          set_target={set_target}
          target={target}
          title={title}
        />
      )}
    </>
  );
}
