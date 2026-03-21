"use client";

import { type ReactNode, useId } from "react";

import { BlobDebugController, BlobDebugPanel } from "@/components/home/hero-blob-debug";
import { useBlobDebugTarget, useEditableShape } from "@/components/home/hero-blob-debug-hooks";
import {
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

export function HeroBlobShell({children, className}: HeroBlobShellProps) {
  const gradientId = useId();
  const glowId = useId();
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
            className="absolute inset-x-[10%] bottom-[18%] h-24 rounded-full bg-[radial-gradient(circle,rgba(133,119,255,0.26),rgba(133,119,255,0)_74%)] blur-3xl"/>
          <div
            className="absolute right-[10%] top-[18%] h-32 w-32 rounded-full bg-[radial-gradient(circle,rgba(118,231,206,0.18),rgba(118,231,206,0)_76%)] blur-3xl"/>
          <div
            className="absolute left-[10%] top-[34%] h-28 w-28 rounded-full bg-[radial-gradient(circle,rgba(255,190,122,0.14),rgba(255,190,122,0)_76%)] blur-3xl"/>
          <div
            className="absolute inset-x-[16%] top-[10%] h-20 rounded-full bg-[radial-gradient(circle,rgba(255,255,255,0.12),rgba(255,255,255,0)_74%)] blur-2xl"/>

          <svg
            aria-hidden="true"
            className="absolute inset-0 h-full w-full pointer-events-none"
            preserveAspectRatio="none"
            viewBox={`0 0 ${OUTER_VIEWBOX_WIDTH} ${OUTER_VIEWBOX_HEIGHT}`}
          >
            <defs>
              <linearGradient id={gradientId} x1="142" x2="900" y1="92" y2="640" gradientUnits="userSpaceOnUse">
                <stop offset="0%" stopColor="rgba(255,255,255,0.44)"/>
                <stop offset="44%" stopColor="rgba(208,221,255,0.26)"/>
                <stop offset="100%" stopColor="rgba(255,255,255,0.12)"/>
              </linearGradient>
              <filter id={glowId} x="-20%" y="-20%" width="140%" height="140%">
                <feGaussianBlur result="blur" stdDeviation="22"/>
                <feColorMatrix
                  in="blur"
                  type="matrix"
                  values="1 0 0 0 0
                          0 1 0 0 0
                          0 0 1 0 0
                          0 0 0 17 -8"
                />
              </filter>
            </defs>

            <path
              d={outer.path}
              fill={outer.debugEnabled ? "rgba(182,191,237,0.58)" : `url(#${gradientId})`}
              filter={`url(#${glowId})`}
              opacity={0.95}
              stroke={outer.debugEnabled ? "rgba(189,200,255,0.68)" : "rgba(175,223,223,0.62)"}
              strokeWidth="2.4"
            />
            <path
              d={createClosedSplinePath(createInnerPoints(outer.points))}
              fill={outer.debugEnabled ? "rgba(88,102,164,0.22)" : "rgba(196,229,230,0.27)"}
              opacity={outer.debugEnabled ? 0.96 : 0.92}
              stroke={outer.debugEnabled ? "rgba(214,221,255,0.32)" : "rgba(255,255,255,0.25)"}
              strokeWidth="1"
            />
          </svg>
        </div>

        <div className="relative z-10 px-14 py-12 text-center sm:px-18 sm:py-16">
          {children}
        </div>

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
            <path
              d={input.path}
              fill={input.debugEnabled ? "rgba(23,34,52,0.72)" : "rgba(255,255,255,0.08)"}
              stroke={input.debugEnabled ? "rgba(176,235,220,0.58)" : "rgba(255,255,255,0.32)"}
              strokeWidth="1.25"
            />
            <path
              d={createClosedSplinePath(createInnerPoints(input.points, 0.93, 0.86))}
              fill={input.debugEnabled ? "rgba(80,122,146,0.24)" : "rgba(255,255,255,0.04)"}
              opacity={input.debugEnabled ? 0.98 : 0.92}
              stroke={input.debugEnabled ? "rgba(206,244,236,0.3)" : "rgba(255,255,255,0.08)"}
              strokeWidth="0.8"
            />
          </svg>
        </div>

        <div className="relative z-10 px-6 py-5">
          {children}
        </div>

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
      </div>

      {input.debugEnabled && activeShape && (
        <BlobDebugPanel
          currentTarget={target}
          onCopy={async () => {
            await navigator.clipboard.writeText(JSON.stringify(activeShape.points, null, 2));
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
