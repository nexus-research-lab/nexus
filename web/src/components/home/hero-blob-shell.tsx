"use client";

import { type ReactNode, useId } from "react";

import { BlobDebugController, BlobDebugPanel } from "@/components/home/hero-blob-debug";
import { useBlobDebugTarget, useEditableShape } from "@/components/home/hero-blob-debug-hooks";
import {
  type BlobPoint,
  createClosedSplinePath,
  createInnerPoints,
  DEFAULT_INPUT_POINTS,
  DEFAULT_OUTER_POINTS,
  INPUT_STORAGE_KEY,
  INPUT_VIEWBOX_HEIGHT,
  INPUT_VIEWBOX_WIDTH,
  OUTER_STORAGE_KEY,
  OUTER_VIEWBOX_HEIGHT,
  OUTER_VIEWBOX_WIDTH,
} from "@/components/home/hero-blob-shape";
import { cn } from "@/lib/utils";

interface HeroBlobShellProps {
  children: ReactNode;
  className?: string;
}

interface HeroInputShellProps {
  children: ReactNode;
  className?: string;
}

interface StaticGlassShellProps {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  fill: string;
  innerFill: string;
  innerPath: string;
  innerStroke: string;
  outerGlowOpacity?: number;
  outerGlowWidth?: number;
  path: string;
  stroke: string;
  viewBoxHeight: number;
  viewBoxWidth: number;
}

interface HeroActionPillShellProps {
  active?: boolean;
  children: ReactNode;
  className?: string;
}

interface HeroActionOrbShellProps {
  active?: boolean;
  children: ReactNode;
  className?: string;
}

const SIDE_PANEL_VIEWBOX_WIDTH = 420;
const SIDE_PANEL_VIEWBOX_HEIGHT = 620;
const SIDE_PANEL_POINTS: BlobPoint[] = [
  {x: 62, y: 102},
  {x: 76, y: 44},
  {x: 162, y: 22},
  {x: 268, y: 28},
  {x: 334, y: 52},
  {x: 352, y: 116},
  {x: 342, y: 204},
  {x: 350, y: 310},
  {x: 346, y: 432},
  {x: 330, y: 534},
  {x: 274, y: 580},
  {x: 184, y: 592},
  {x: 104, y: 576},
  {x: 70, y: 520},
  {x: 56, y: 428},
  {x: 52, y: 300},
  {x: 58, y: 176},
];
const SIDE_PANEL_PATH = createClosedSplinePath(SIDE_PANEL_POINTS);
const SIDE_PANEL_INNER_PATH = createClosedSplinePath(createInnerPoints(SIDE_PANEL_POINTS, 0.958, 0.95));

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
  children,
  className,
  contentClassName,
  fill,
  innerFill,
  innerPath,
  innerStroke,
  outerGlowOpacity = 0.86,
  outerGlowWidth = 18,
  path,
  stroke,
  viewBoxHeight,
  viewBoxWidth,
}: StaticGlassShellProps) {
  const glowGradientId = useId();
  const glowId = useId();

  return (
    <div className={cn("relative isolate overflow-visible", className)}>
      <div className="absolute inset-[-14%] z-0 pointer-events-none">
        <div
          className="absolute inset-0 blur-[44px]"
          style={{
            background: `radial-gradient(28% 22% at 20% 24%, rgba(255,190,122,0.12), rgba(255,190,122,0) 76%),
              radial-gradient(24% 24% at 82% 18%, rgba(118,231,206,0.12), rgba(118,231,206,0) 78%),
              radial-gradient(34% 26% at 46% 82%, rgba(133,119,255,0.14), rgba(133,119,255,0) 74%),
              radial-gradient(40% 20% at 52% 12%, rgba(255,255,255,0.12), rgba(255,255,255,0) 72%)`,
          }}
        />
      </div>

      <svg
        aria-hidden="true"
        className="absolute inset-0 z-0 h-full w-full pointer-events-none"
        preserveAspectRatio="none"
        viewBox={`0 0 ${viewBoxWidth} ${viewBoxHeight}`}
      >
        <defs>
          <linearGradient id={glowGradientId} x1="42" x2={viewBoxWidth - 28} y1="26" y2={viewBoxHeight - 18} gradientUnits="userSpaceOnUse">
            <stop offset="0%" stopColor="rgba(255,255,255,0.78)"/>
            <stop offset="34%" stopColor="rgba(255,255,255,0.34)"/>
            <stop offset="74%" stopColor="rgba(211,224,248,0.26)"/>
            <stop offset="100%" stopColor="rgba(255,255,255,0.18)"/>
          </linearGradient>
          <filter id={glowId} x="-20%" y="-80%" width="140%" height="260%">
            <feGaussianBlur stdDeviation="6"/>
          </filter>
        </defs>

        <path
          d={path}
          fill="none"
          filter={`url(#${glowId})`}
          opacity={outerGlowOpacity}
          stroke={`url(#${glowGradientId})`}
          strokeWidth={outerGlowWidth}
        />
        <path
          d={innerPath}
          fill="none"
          filter={`url(#${glowId})`}
          opacity="0.78"
          stroke="rgba(255,255,255,0.22)"
          strokeWidth={Math.max(outerGlowWidth - 4, 10)}
        />
        <path
          d={path}
          fill={fill}
          opacity="0.94"
          stroke={stroke}
          strokeWidth="2"
        />
        <path
          d={innerPath}
          fill={innerFill}
          opacity="0.92"
          stroke={innerStroke}
          strokeWidth="4"
        />
      </svg>

      <div className={cn("relative z-10", contentClassName)}>
        {children}
      </div>
    </div>
  );
}

export function HeroSidePanelShell({children, className}: HeroBlobShellProps) {
  return (
    <StaticGlassShell
      className={cn("w-[360px]", className)}
      contentClassName="px-6 py-7"
      fill="rgba(255,255,255,0.08)"
      innerFill="rgba(255,255,255,0.04)"
      innerPath={SIDE_PANEL_INNER_PATH}
      innerStroke="rgba(255,255,255,0.08)"
      path={SIDE_PANEL_PATH}
      stroke="rgba(255,255,255,0.28)"
      viewBoxHeight={SIDE_PANEL_VIEWBOX_HEIGHT}
      viewBoxWidth={SIDE_PANEL_VIEWBOX_WIDTH}
    >
      {children}
    </StaticGlassShell>
  );
}

export function HeroActionPillShell({
  active = false,
  children,
  className,
}: HeroActionPillShellProps) {
  return (
    <StaticGlassShell
      className={cn("h-11 min-w-[108px]", className)}
      contentClassName="flex h-full items-center justify-center px-5"
      fill={active ? "rgba(255,255,255,0.16)" : "rgba(255,255,255,0.10)"}
      innerFill={active ? "rgba(255,255,255,0.08)" : "rgba(255,255,255,0.04)"}
      innerPath={ACTION_PILL_INNER_PATH}
      innerStroke={active ? "rgba(255,255,255,0.12)" : "rgba(255,255,255,0.08)"}
      outerGlowOpacity={active ? 0.92 : 0.78}
      outerGlowWidth={14}
      path={ACTION_PILL_PATH}
      stroke={active ? "rgba(255,255,255,0.34)" : "rgba(255,255,255,0.24)"}
      viewBoxHeight={ACTION_PILL_VIEWBOX_HEIGHT}
      viewBoxWidth={ACTION_PILL_VIEWBOX_WIDTH}
    >
      {children}
    </StaticGlassShell>
  );
}

export function HeroActionOrbShell({
  active = false,
  children,
  className,
}: HeroActionOrbShellProps) {
  return (
    <StaticGlassShell
      className={cn("h-11 w-11", className)}
      contentClassName="flex h-full items-center justify-center"
      fill={active ? "rgba(190,240,202,0.22)" : "rgba(255,255,255,0.12)"}
      innerFill={active ? "rgba(191,240,202,0.12)" : "rgba(255,255,255,0.05)"}
      innerPath={ACTION_ORB_INNER_PATH}
      innerStroke={active ? "rgba(180,235,194,0.18)" : "rgba(255,255,255,0.08)"}
      outerGlowOpacity={active ? 0.9 : 0.76}
      outerGlowWidth={12}
      path={ACTION_ORB_PATH}
      stroke={active ? "rgba(191,240,202,0.44)" : "rgba(255,255,255,0.24)"}
      viewBoxHeight={ACTION_ORB_VIEWBOX_HEIGHT}
      viewBoxWidth={ACTION_ORB_VIEWBOX_WIDTH}
    >
      {children}
    </StaticGlassShell>
  );
}

export function HeroBlobShell({children, className}: HeroBlobShellProps) {
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
      <div className={cn("relative isolate w-full max-w-[980px]", className)}>
        <div className="absolute inset-[-20%] z-0">
          <div
            className="absolute inset-0 pointer-events-none blur-[56px]"
            style={{
              background: `radial-gradient(30% 16% at 50% 82%, rgba(133,119,255,0.26), rgba(133,119,255,0) 74%),
                radial-gradient(12% 20% at 86% 22%, rgba(118,231,206,0.18), rgba(118,231,206,0) 76%),
                radial-gradient(12% 18% at 14% 38%, rgba(255,190,122,0.14), rgba(255,190,122,0) 76%),
                radial-gradient(40% 12% at 50% 12%, rgba(255,255,255,0.12), rgba(255,255,255,0) 74%)`,
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
                <stop offset="0%" stopColor="rgba(255,255,255,0.08)"/>
                <stop offset="44%" stopColor="rgba(255,255,255,0.08)"/>
                <stop offset="100%" stopColor="rgba(255,255,255,0.08)"/>
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
              fill={outer.debugEnabled ? "rgba(88,102,164,0.22)" : "rgba(255,255,255,0.04)"}
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
            active={target === "hero"}
            color="rgba(122,108,255,0.9)"
            currentTarget={target}
            enabled={outer.debugEnabled}
            fill="transparent"
            onCopy={async () => {
              await navigator.clipboard.writeText(outer.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
            }}
            onPathDoubleClick={outer.handlePathDoubleClick}
            onPointPointerDown={outer.handlePointPointerDown}
            onPointPointerUp={outer.handlePointPointerUp}
            onReset={() => {
              localStorage.removeItem(OUTER_STORAGE_KEY);
              outer.setPoints(DEFAULT_OUTER_POINTS);
            }}
            panelClassName="bottom-4 left-4"
            path={outer.path}
            points={outer.points}
            setTarget={setTarget}
            showPanel={false}
            stroke="transparent"
            strokeWidth={10}
            svgRef={outer.svgRef}
            target="hero"
            title="Hero Shape"
            viewBoxHeight={OUTER_VIEWBOX_HEIGHT}
            viewBoxWidth={OUTER_VIEWBOX_WIDTH}
          />
        )}
      </div>

      {outer.debugEnabled && activeShape && (
        <BlobDebugPanel
          currentTarget={target}
          onCopy={async () => {
            await navigator.clipboard.writeText(activeShape.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
          }}
          onReset={() => {
            localStorage.removeItem(OUTER_STORAGE_KEY);
            outer.setPoints(DEFAULT_OUTER_POINTS);
          }}
          panelClassName="bottom-4 left-4"
          points={activeShape.points}
          setTarget={setTarget}
          target={target}
          title="Blob Shape"
        />
      )}
    </>
  );
}

export function HeroInputShell({children, className}: HeroInputShellProps) {
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
      <div className={cn("relative isolate w-full", className)}>
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
            active={target === "input"}
            color="rgba(118,231,206,0.92)"
            currentTarget={target}
            enabled={input.debugEnabled}
            fill="transparent"
            onCopy={async () => {
              await navigator.clipboard.writeText(input.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
            }}
            onPathDoubleClick={input.handlePathDoubleClick}
            onPointPointerDown={input.handlePointPointerDown}
            onPointPointerUp={input.handlePointPointerUp}
            onReset={() => {
              localStorage.removeItem(INPUT_STORAGE_KEY);
              input.setPoints(DEFAULT_INPUT_POINTS);
            }}
            panelClassName="bottom-4 right-4"
            path={input.path}
            points={input.points}
            setTarget={setTarget}
            showPanel={false}
            stroke="transparent"
            strokeWidth={10}
            svgRef={input.svgRef}
            target="input"
            title="Input Shape"
            viewBoxHeight={INPUT_VIEWBOX_HEIGHT}
            viewBoxWidth={INPUT_VIEWBOX_WIDTH}
          />
        )}
      </div>

      {input.debugEnabled && activeShape && (
        <BlobDebugPanel
          currentTarget={target}
          onCopy={async () => {
            await navigator.clipboard.writeText(activeShape.points.map(p => `{\"x\":${p.x},\"y\":${p.y}}`).join(",\n"));
          }}
          onReset={() => {
            localStorage.removeItem(INPUT_STORAGE_KEY);
            input.setPoints(DEFAULT_INPUT_POINTS);
          }}
          panelClassName="bottom-4 left-4"
          points={activeShape.points}
          setTarget={setTarget}
          target={target}
          title="Blob Shape"
        />
      )}
    </>
  );
}
