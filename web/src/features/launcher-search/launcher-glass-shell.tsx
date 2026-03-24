"use client";

import { useId } from "react";

import {
  type BlobPoint,
  createClosedSplinePath,
  createInnerPoints,
  DEFAULT_INPUT_POINTS,
  DEFAULT_OUTER_POINTS,
  DEFAULT_SIDE_PANEL_POINTS,
  INPUT_STORAGE_KEY,
  INPUT_VIEWBOX_HEIGHT,
  INPUT_VIEWBOX_WIDTH,
  OUTER_STORAGE_KEY,
  OUTER_VIEWBOX_HEIGHT,
  OUTER_VIEWBOX_WIDTH,
  SIDE_PANEL_STORAGE_KEY,
  SIDE_PANEL_VIEWBOX_HEIGHT,
  SIDE_PANEL_VIEWBOX_WIDTH,
} from "@/features/launcher-search/launcher-blob-shape";
import { cn } from "@/lib/utils";
import {
  HeroActionOrbShellProps,
  HeroActionPillShellProps,
  HeroBlobShellProps,
  HeroInputShellProps,
  StaticGlassShellProps,
} from "@/types/launcher-ui";

import { BlobDebugController, BlobDebugPanel } from "./launcher-blob-debug";
import { useBlobDebugTarget, useEditableShape } from "./launcher-blob-debug-hooks";

const ACTION_PILL_VIEWBOX_WIDTH = 220;
const ACTION_PILL_VIEWBOX_HEIGHT = 86;
const ACTION_PILL_POINTS: BlobPoint[] = [
  {x: 26, y: 55},
  {x: 28, y: 24},
  {x: 64, y: 16},
  {x: 114, y: 14},
  {x: 164, y: 17},
  {x: 194, y: 24},
  {x: 208, y: 39},
  {x: 202, y: 58},
  {x: 170, y: 66},
  {x: 110, y: 68},
  {x: 60, y: 65},
];
const ACTION_PILL_PATH = createClosedSplinePath(ACTION_PILL_POINTS);
const ACTION_PILL_INNER_PATH = createClosedSplinePath(createInnerPoints(ACTION_PILL_POINTS, 0.968, 0.92));

const ACTION_ORB_VIEWBOX_WIDTH = 92;
const ACTION_ORB_VIEWBOX_HEIGHT = 92;
const ACTION_ORB_POINTS: BlobPoint[] = [
  {x: 18, y: 57},
  {x: 20, y: 28},
  {x: 38, y: 16},
  {x: 64, y: 18},
  {x: 76, y: 36},
  {x: 74, y: 66},
  {x: 54, y: 78},
  {x: 28, y: 74},
];
const ACTION_ORB_PATH = createClosedSplinePath(ACTION_ORB_POINTS);
const ACTION_ORB_INNER_PATH = createClosedSplinePath(createInnerPoints(ACTION_ORB_POINTS, 0.93, 0.93));

function StaticGlassShell({
  aura_background = `radial-gradient(28% 22% at 20% 24%, rgba(255,190,122,0.12), rgba(255,190,122,0) 76%),
    radial-gradient(24% 24% at 82% 18%, rgba(118,231,206,0.12), rgba(118,231,206,0) 78%),
    radial-gradient(34% 26% at 46% 82%, rgba(133,119,255,0.14), rgba(133,119,255,0) 74%),
    radial-gradient(40% 20% at 52% 12%, rgba(255,255,255,0.12), rgba(255,255,255,0) 72%)`,
  aura_blur_class_name = "blur-[44px]",
  children,
  class_name,
  content_class_name,
  fill,
  fill_gradient_stops,
  glow_blur_deviation = 6,
  inner_fill,
  inner_fill_gradient_stops,
  inner_glow_opacity = 0.78,
  inner_path,
  inner_stroke,
  outer_glow_opacity = 0.86,
  outer_glow_width = 18,
  path,
  stroke,
  svg_overlay,
  view_box_height,
  view_box_width,
}: StaticGlassShellProps) {
  const glowGradientId = useId();
  const glowId = useId();
  const fillGradientId = useId();
  const innerFillGradientId = useId();
  const surfaceFill = fill_gradient_stops ? `url(#${fillGradientId})` : fill;
  const innerSurfaceFill = inner_fill_gradient_stops ? `url(#${innerFillGradientId})` : inner_fill;

  return (
    <div className={cn("relative isolate overflow-visible", class_name)}>
      <div className="absolute inset-[-14%] z-0 pointer-events-none">
        {aura_background ? (
          <div
            className={cn("absolute inset-0", aura_blur_class_name)}
            style={{ background: aura_background }}
          />
        ) : null}
      </div>

      <svg
        aria-hidden="true"
        className="absolute inset-0 z-0 h-full w-full pointer-events-none"
        preserveAspectRatio="none"
        viewBox={`0 0 ${view_box_width} ${view_box_height}`}
      >
        <defs>
          <linearGradient id={glowGradientId} x1="42" x2={view_box_width - 28} y1="26" y2={view_box_height - 18} gradientUnits="userSpaceOnUse">
            <stop offset="0%" stopColor="rgba(255,255,255,0.78)"/>
            <stop offset="34%" stopColor="rgba(255,255,255,0.34)"/>
            <stop offset="74%" stopColor="rgba(211,224,248,0.26)"/>
            <stop offset="100%" stopColor="rgba(255,255,255,0.18)"/>
          </linearGradient>
          <filter id={glowId} x="-20%" y="-80%" width="140%" height="260%">
            <feGaussianBlur stdDeviation={glow_blur_deviation}/>
          </filter>
          {fill_gradient_stops ? (
            <linearGradient id={fillGradientId} x1="28" x2={view_box_width - 20} y1="24" y2={view_box_height - 18} gradientUnits="userSpaceOnUse">
              {fill_gradient_stops.map((stop) => (
                <stop key={`fill-${stop.offset}-${stop.color}`} offset={stop.offset} stopColor={stop.color}/>
              ))}
            </linearGradient>
          ) : null}
          {inner_fill_gradient_stops ? (
            <linearGradient id={innerFillGradientId} x1="42" x2={view_box_width - 36} y1="32" y2={view_box_height - 22} gradientUnits="userSpaceOnUse">
              {inner_fill_gradient_stops.map((stop) => (
                <stop key={`inner-${stop.offset}-${stop.color}`} offset={stop.offset} stopColor={stop.color}/>
              ))}
            </linearGradient>
          ) : null}
        </defs>

        <path
          d={path}
          fill="none"
          filter={`url(#${glowId})`}
          opacity={outer_glow_opacity}
          stroke={`url(#${glowGradientId})`}
          strokeWidth={outer_glow_width}
        />
        <path
          d={inner_path}
          fill="none"
          filter={`url(#${glowId})`}
          opacity={inner_glow_opacity}
          stroke="rgba(255,255,255,0.22)"
          strokeWidth={Math.max(outer_glow_width - 4, 10)}
        />
        <path
          d={path}
          fill={surfaceFill}
          opacity="0.94"
          stroke={stroke}
          strokeWidth="2"
        />
        <path
          d={inner_path}
          fill={innerSurfaceFill}
          opacity="0.92"
          stroke={inner_stroke}
          strokeWidth="4"
        />
      </svg>

      {svg_overlay}

      <div className={cn("relative z-10", content_class_name)}>
        {children}
      </div>
    </div>
  );
}

export function HeroSidePanelShell({ children, class_name }: HeroBlobShellProps) {
  const panel = useEditableShape({
    defaultPoints: DEFAULT_SIDE_PANEL_POINTS,
    storageKey: SIDE_PANEL_STORAGE_KEY,
    viewBoxHeight: SIDE_PANEL_VIEWBOX_HEIGHT,
    viewBoxWidth: SIDE_PANEL_VIEWBOX_WIDTH,
  });
  const {setTarget, target} = useBlobDebugTarget();
  const activeShape = target === "panel" ? panel : null;
  const innerPath = createClosedSplinePath(createInnerPoints(panel.points, 0.958, 0.95));

  return (
    <>
        <StaticGlassShell
        class_name={cn("w-[280px]", class_name)}
        content_class_name="px-6 py-7"
        aura_background={`radial-gradient(24% 20% at 18% 28%, rgba(255,184,124,0.10), rgba(255,184,124,0) 78%),
          radial-gradient(24% 20% at 80% 18%, rgba(110,228,214,0.10), rgba(110,228,214,0) 78%),
          radial-gradient(32% 18% at 46% 92%, rgba(128,118,255,0.12), rgba(128,118,255,0) 76%),
          radial-gradient(40% 18% at 52% 8%, rgba(255,255,255,0.08), rgba(255,255,255,0) 72%)`}
        aura_blur_class_name="blur-[34px]"
        fill={panel.debugEnabled ? "rgba(37,55,88,0.5)" : "rgba(214,224,246,0.12)"}
        fill_gradient_stops={panel.debugEnabled ? undefined : [
          { offset: "0%", color: "rgba(245,248,255,0.18)" },
          { offset: "42%", color: "rgba(212,224,248,0.12)" },
          { offset: "100%", color: "rgba(182,200,235,0.14)" },
        ]}
        glow_blur_deviation={5}
        inner_fill={panel.debugEnabled ? "rgba(255,255,255,0.08)" : "rgba(255,255,255,0.05)"}
        inner_fill_gradient_stops={panel.debugEnabled ? undefined : [
          { offset: "0%", color: "rgba(250,252,255,0.08)" },
          { offset: "52%", color: "rgba(217,229,249,0.04)" },
          { offset: "100%", color: "rgba(200,214,240,0.07)" },
        ]}
        inner_glow_opacity={panel.debugEnabled ? 0.78 : 0.42}
        inner_path={innerPath}
        inner_stroke={panel.debugEnabled ? "rgba(170,226,255,0.22)" : "rgba(255,255,255,0.09)"}
        outer_glow_opacity={panel.debugEnabled ? 0.86 : 0.5}
        outer_glow_width={panel.debugEnabled ? 18 : 13}
        path={panel.path}
        stroke={panel.debugEnabled ? "rgba(170,226,255,0.42)" : "rgba(255,255,255,0.22)"}
        svg_overlay={panel.debugEnabled ? (
          <BlobDebugController
            is_active={target === "panel"}
            color="rgba(170,226,255,0.92)"
            current_target={target}
            debug_enabled={panel.debugEnabled}
            fill="transparent"
            on_copy={async () => {
              await navigator.clipboard.writeText(panel.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
            }}
            on_path_double_click={panel.handlePathDoubleClick}
            on_point_pointer_down={panel.handlePointPointerDown}
            on_point_pointer_up={panel.handlePointPointerUp}
            on_reset={() => {
              localStorage.removeItem(SIDE_PANEL_STORAGE_KEY);
              panel.setPoints(DEFAULT_SIDE_PANEL_POINTS);
            }}
            panel_class_name="bottom-4 left-4"
            path={panel.path}
            points={panel.points}
            set_target={setTarget}
            show_panel={false}
            stroke="transparent"
            stroke_width={10}
            svg_ref={panel.svgRef}
            target="panel"
            title="Panel Shape"
            view_box_height={SIDE_PANEL_VIEWBOX_HEIGHT}
            view_box_width={SIDE_PANEL_VIEWBOX_WIDTH}
          />
        ) : null}
        view_box_height={SIDE_PANEL_VIEWBOX_HEIGHT}
        view_box_width={SIDE_PANEL_VIEWBOX_WIDTH}
      >
        {children}
      </StaticGlassShell>

      {panel.debugEnabled && activeShape && (
        <BlobDebugPanel
          current_target={target}
          on_copy={async () => {
            await navigator.clipboard.writeText(activeShape.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
          }}
          on_reset={() => {
            localStorage.removeItem(SIDE_PANEL_STORAGE_KEY);
            panel.setPoints(DEFAULT_SIDE_PANEL_POINTS);
          }}
          panel_class_name="bottom-4 left-4"
          points={activeShape.points}
          set_target={setTarget}
          target={target}
          title="Blob Shape"
        />
      )}
    </>
  );
}

export function HeroActionPillShell({
  is_active = false,
  children,
  class_name,
}: HeroActionPillShellProps) {
  return (
    <StaticGlassShell
      class_name={cn("h-11 min-w-[108px]", class_name)}
      content_class_name="flex h-full items-center justify-center px-5"
      aura_background={is_active
        ? "radial-gradient(48% 38% at 50% 50%, rgba(255,255,255,0.12), rgba(255,255,255,0) 72%)"
        : ""}
      aura_blur_class_name="blur-[18px]"
      fill={is_active ? "rgba(219,228,246,0.16)" : "rgba(204,216,239,0.12)"}
      fill_gradient_stops={[
        { offset: "0%", color: is_active ? "rgba(247,250,255,0.24)" : "rgba(238,244,255,0.18)" },
        { offset: "100%", color: is_active ? "rgba(203,217,241,0.14)" : "rgba(188,204,233,0.12)" },
      ]}
      glow_blur_deviation={4}
      inner_fill={is_active ? "rgba(255,255,255,0.08)" : "rgba(255,255,255,0.04)"}
      inner_path={ACTION_PILL_INNER_PATH}
      inner_glow_opacity={is_active ? 0.3 : 0.22}
      inner_stroke={is_active ? "rgba(255,255,255,0.12)" : "rgba(255,255,255,0.08)"}
      outer_glow_opacity={is_active ? 0.44 : 0.28}
      outer_glow_width={10}
      path={ACTION_PILL_PATH}
      stroke={is_active ? "rgba(255,255,255,0.22)" : "rgba(255,255,255,0.14)"}
      view_box_height={ACTION_PILL_VIEWBOX_HEIGHT}
      view_box_width={ACTION_PILL_VIEWBOX_WIDTH}
    >
      {children}
    </StaticGlassShell>
  );
}

export function HeroActionOrbShell({
  is_active = false,
  children,
  class_name,
}: HeroActionOrbShellProps) {
  return (
    <StaticGlassShell
      class_name={cn("h-11 w-11", class_name)}
      content_class_name="flex h-full items-center justify-center"
      aura_background={is_active
        ? "radial-gradient(54% 54% at 50% 50%, rgba(171,238,194,0.22), rgba(171,238,194,0) 70%)"
        : ""}
      aura_blur_class_name="blur-[18px]"
      fill={is_active ? "rgba(176,235,192,0.22)" : "rgba(204,216,239,0.12)"}
      fill_gradient_stops={[
        { offset: "0%", color: is_active ? "rgba(229,252,235,0.30)" : "rgba(238,244,255,0.18)" },
        { offset: "100%", color: is_active ? "rgba(150,222,170,0.20)" : "rgba(188,204,233,0.12)" },
      ]}
      glow_blur_deviation={4}
      inner_fill={is_active ? "rgba(191,240,202,0.10)" : "rgba(255,255,255,0.05)"}
      inner_path={ACTION_ORB_INNER_PATH}
      inner_glow_opacity={is_active ? 0.3 : 0.2}
      inner_stroke={is_active ? "rgba(180,235,194,0.18)" : "rgba(255,255,255,0.08)"}
      outer_glow_opacity={is_active ? 0.46 : 0.28}
      outer_glow_width={10}
      path={ACTION_ORB_PATH}
      stroke={is_active ? "rgba(191,240,202,0.26)" : "rgba(255,255,255,0.14)"}
      view_box_height={ACTION_ORB_VIEWBOX_HEIGHT}
      view_box_width={ACTION_ORB_VIEWBOX_WIDTH}
    >
      {children}
    </StaticGlassShell>
  );
}

export function HeroBlobShell({ children, class_name }: HeroBlobShellProps) {
  const gradientId = useId();
  const outerEdgeGlowGradientId = useId();
  const outerEdgeGlowId = useId();
  const outer = useEditableShape({
    defaultPoints: DEFAULT_OUTER_POINTS,
    storageKey: OUTER_STORAGE_KEY,
    viewBoxHeight: OUTER_VIEWBOX_HEIGHT,
    viewBoxWidth: OUTER_VIEWBOX_WIDTH,
  });
  const {setTarget, target} = useBlobDebugTarget();
  const activeShape = target === "hero" ? outer : null;

  return (
    <>
      <div className={cn("relative isolate w-full max-w-[980px]", class_name)}>
        <div className="absolute inset-[-20%] z-0 pointer-events-none">
          <div
            className="absolute inset-0 pointer-events-none blur-[56px]"
            style={{
              background: `radial-gradient(30% 16% at 50% 82%, rgba(133,119,255,0.30), rgba(133,119,255,0) 74%),
                radial-gradient(12% 20% at 86% 22%, rgba(118,231,206,0.18), rgba(118,231,206,0) 76%),
                radial-gradient(12% 18% at 14% 38%, rgba(255,190,122,0.14), rgba(255,190,122,0) 76%),
                radial-gradient(40% 12% at 50% 12%, rgba(255,255,255,0.14), rgba(255,255,255,0) 74%)`,
            }}
          />

          <svg
            aria-hidden="true"
            className="absolute inset-0 h-full w-full pointer-events-none"
            preserveAspectRatio="none"
            viewBox={`0 0 ${OUTER_VIEWBOX_WIDTH} ${OUTER_VIEWBOX_HEIGHT}`}
          >
            <defs>
              <linearGradient id={gradientId} x1="142" x2="900" y1="92" y2="640" gradientUnits="userSpaceOnUse">
                <stop offset="0%" stopColor="rgba(229,237,255,0.12)"/>
                <stop offset="44%" stopColor="rgba(214,225,248,0.11)"/>
                <stop offset="100%" stopColor="rgba(203,216,241,0.12)"/>
              </linearGradient>
              <linearGradient id={outerEdgeGlowGradientId} x1="176" x2="888" y1="74" y2="674" gradientUnits="userSpaceOnUse">
                <stop offset="0%" stopColor="rgba(255,255,255,0.78)"/>
                <stop offset="34%" stopColor="rgba(255,255,255,0.34)"/>
                <stop offset="74%" stopColor="rgba(211,224,248,0.26)"/>
                <stop offset="100%" stopColor="rgba(255,255,255,0.18)"/>
              </linearGradient>
              <filter id={outerEdgeGlowId} x="-20%" y="-80%" width="140%" height="260%">
                <feGaussianBlur stdDeviation="6"/>
              </filter>
            </defs>

            <path
              d={outer.path}
              fill="none"
              filter={`url(#${outerEdgeGlowId})`}
              opacity={outer.debugEnabled ? 0 : 0.88}
              stroke={`url(#${outerEdgeGlowGradientId})`}
              strokeWidth="18"
            />
            <path
              d={createClosedSplinePath(createInnerPoints(outer.points, 0.985, 0.982))}
              fill="none"
              filter={`url(#${outerEdgeGlowId})`}
              opacity={outer.debugEnabled ? 0 : 0.78}
              stroke="rgba(255,255,255,0.22)"
              strokeWidth="14"
            />
            <path
              d={outer.path}
              fill={outer.debugEnabled ? "rgba(182,191,237,0.58)" : `url(#${gradientId})`}
              opacity={0.92}
              stroke={outer.debugEnabled ? "rgba(189,200,255,0.68)" : "rgba(255,255,255,0.32)"}
              strokeWidth="2"
            />
            <path
              d={createClosedSplinePath(createInnerPoints(outer.points, 0.992, 0.99))}
              fill={outer.debugEnabled ? "rgba(88,102,164,0.22)" : "rgba(216,226,247,0.06)"}
              opacity={outer.debugEnabled ? 0.96 : 0.92}
              stroke={outer.debugEnabled ? "rgba(214,221,255,0.32)" : "rgba(255,255,255,0.08)"}
              strokeWidth="4"
            />
          </svg>
        </div>

        <div className="relative z-10 px-14 py-12 text-center sm:px-18 sm:py-16">
          {children}
        </div>

        {outer.debugEnabled && (
          <BlobDebugController
            is_active={target === "hero"}
            color="rgba(122,108,255,0.9)"
            current_target={target}
            debug_enabled={outer.debugEnabled}
            fill="transparent"
            on_copy={async () => {
              await navigator.clipboard.writeText(outer.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
            }}
            on_path_double_click={outer.handlePathDoubleClick}
            on_point_pointer_down={outer.handlePointPointerDown}
            on_point_pointer_up={outer.handlePointPointerUp}
            on_reset={() => {
              localStorage.removeItem(OUTER_STORAGE_KEY);
              outer.setPoints(DEFAULT_OUTER_POINTS);
            }}
            panel_class_name="bottom-4 left-4"
            path={outer.path}
            points={outer.points}
            set_target={setTarget}
            show_panel={false}
            stroke="transparent"
            stroke_width={10}
            svg_ref={outer.svgRef}
            target="hero"
            title="Hero Shape"
            view_box_height={OUTER_VIEWBOX_HEIGHT}
            view_box_width={OUTER_VIEWBOX_WIDTH}
          />
        )}
      </div>

      {outer.debugEnabled && activeShape && (
        <BlobDebugPanel
          current_target={target}
          on_copy={async () => {
            await navigator.clipboard.writeText(activeShape.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
          }}
          on_reset={() => {
            localStorage.removeItem(OUTER_STORAGE_KEY);
            outer.setPoints(DEFAULT_OUTER_POINTS);
          }}
          panel_class_name="bottom-4 left-4"
          points={activeShape.points}
          set_target={setTarget}
          target={target}
          title="Blob Shape"
        />
      )}
    </>
  );
}

export function HeroInputShell({ children, class_name }: HeroInputShellProps) {
  const inputGlowGradientId = useId();
  const inputGlowId = useId();
  const input = useEditableShape({
    defaultPoints: DEFAULT_INPUT_POINTS,
    storageKey: INPUT_STORAGE_KEY,
    viewBoxHeight: INPUT_VIEWBOX_HEIGHT,
    viewBoxWidth: INPUT_VIEWBOX_WIDTH,
  });
  const {setTarget, target} = useBlobDebugTarget();
  const activeShape = target === "input" ? input : null;

  return (
    <>
      <div className={cn("relative isolate w-full", class_name)}>
        <div className="absolute inset-[-6%] z-0">
          <svg
            aria-hidden="true"
            className="absolute inset-0 h-full w-full pointer-events-none"
            preserveAspectRatio="none"
            viewBox={`0 0 ${INPUT_VIEWBOX_WIDTH} ${INPUT_VIEWBOX_HEIGHT}`}
          >
            <defs>
              <linearGradient id={inputGlowGradientId} x1="42" x2="712" y1="34" y2="142" gradientUnits="userSpaceOnUse">
                <stop offset="0%" stopColor="rgba(255,255,255,0.78)"/>
                <stop offset="34%" stopColor="rgba(255,255,255,0.34)"/>
                <stop offset="74%" stopColor="rgba(211,224,248,0.26)"/>
                <stop offset="100%" stopColor="rgba(255,255,255,0.18)"/>
              </linearGradient>
              <filter id={inputGlowId} x="-20%" y="-80%" width="140%" height="260%">
                <feGaussianBlur stdDeviation="6"/>
              </filter>
            </defs>

            <path
              d={input.path}
              fill="none"
              filter={`url(#${inputGlowId})`}
              opacity={input.debugEnabled ? 0 : 0.88}
              stroke={`url(#${inputGlowGradientId})`}
              strokeWidth="18"
            />
            <path
              d={createClosedSplinePath(createInnerPoints(input.points, 0.965, 0.92))}
              fill="none"
              filter={`url(#${inputGlowId})`}
              opacity={input.debugEnabled ? 0 : 0.78}
              stroke="rgba(255,255,255,0.22)"
              strokeWidth="14"
            />
            <path
              d={input.path}
              fill={input.debugEnabled ? "rgba(23,34,52,0.72)" : "rgba(255,255,255,0.08)"}
              stroke={input.debugEnabled ? "rgba(176,235,220,0.58)" : "rgba(255,255,255,0.32)"}
              strokeWidth="2"
            />
            <path
              d={createClosedSplinePath(createInnerPoints(input.points, 0.93, 0.86))}
              fill={input.debugEnabled ? "rgba(80,122,146,0.24)" : "rgba(255,255,255,0.04)"}
              opacity={input.debugEnabled ? 0.98 : 0.92}
              stroke={input.debugEnabled ? "rgba(206,244,236,0.3)" : "rgba(255,255,255,0.08)"}
              strokeWidth="4"
            />
          </svg>
        </div>

        <div className="relative z-10 px-6 py-5">
          {children}
        </div>

        {input.debugEnabled && (
          <BlobDebugController
            is_active={target === "input"}
            color="rgba(118,231,206,0.92)"
            current_target={target}
            debug_enabled={input.debugEnabled}
            fill="transparent"
            on_copy={async () => {
              await navigator.clipboard.writeText(input.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
            }}
            on_path_double_click={input.handlePathDoubleClick}
            on_point_pointer_down={input.handlePointPointerDown}
            on_point_pointer_up={input.handlePointPointerUp}
            on_reset={() => {
              localStorage.removeItem(INPUT_STORAGE_KEY);
              input.setPoints(DEFAULT_INPUT_POINTS);
            }}
            panel_class_name="bottom-4 right-4"
            path={input.path}
            points={input.points}
            set_target={setTarget}
            show_panel={false}
            stroke="transparent"
            stroke_width={10}
            svg_ref={input.svgRef}
            target="input"
            title="Input Shape"
            view_box_height={INPUT_VIEWBOX_HEIGHT}
            view_box_width={INPUT_VIEWBOX_WIDTH}
          />
        )}
      </div>

      {input.debugEnabled && activeShape && (
        <BlobDebugPanel
          current_target={target}
          on_copy={async () => {
            await navigator.clipboard.writeText(activeShape.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
          }}
          on_reset={() => {
            localStorage.removeItem(INPUT_STORAGE_KEY);
            input.setPoints(DEFAULT_INPUT_POINTS);
          }}
          panel_class_name="bottom-4 left-4"
          points={activeShape.points}
          set_target={setTarget}
          target={target}
          title="Blob Shape"
        />
      )}
    </>
  );
}
