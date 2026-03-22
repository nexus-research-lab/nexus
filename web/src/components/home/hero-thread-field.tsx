"use client";

import { motion } from "framer-motion";
import { type PointerEvent as ReactPointerEvent, type RefObject, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { type SpotlightToken } from "@/components/home/agent-pile";
import { BlobDebugPanel, DebugPortalHandles, useDebugSvgRect } from "@/components/home/hero-blob-debug";
import { useBlobDebugEnabled, useBlobDebugTarget } from "@/components/home/hero-blob-debug-hooks";
import { clamp, type BlobPoint } from "@/components/home/hero-blob-shape";
import {
  DEFAULT_THREAD_BUNDLES,
  parseThreadBundles,
  serializeThreadBundles,
  THREAD_POINT_KEYS,
  THREAD_STORAGE_KEY,
  type ThreadBundleConfig,
  type ThreadPointKey,
} from "@/components/home/hero-thread-shape";
import { cn } from "@/lib/utils";

interface HeroThreadFieldProps {
  sceneRef: RefObject<HTMLDivElement | null>;
  tokens: SpotlightToken[];
}

interface OrbitTokenLayout {
  className: string;
  delay: number;
}

interface Point {
  x: number;
  y: number;
}

interface HeroRectMetrics {
  height: number;
  left: number;
  top: number;
  width: number;
}

interface SceneMetrics {
  height: number;
  heroRect: HeroRectMetrics | null;
  width: number;
}

interface BundleScenePoints {
  control1: Point;
  control2: Point;
  end: Point;
  start: Point;
}

type ThreadHandleState = {
  bundleIndex: number;
  pointKey: ThreadPointKey;
  pointerId: number;
} | null;

const ORBIT_TOKEN_LAYOUTS: OrbitTokenLayout[] = [
  { className: "left-[12%] top-[31%]", delay: 0.1 },
  { className: "left-[68%] top-[8%]", delay: 0.6 },
  { className: "right-[14%] top-[18%]", delay: 0.9 },
  { className: "right-[24%] top-[58%]", delay: 1.3 },
  { className: "left-[26%] top-[70%]", delay: 1.7 },
];

const THREAD_POINT_LABELS: Record<ThreadPointKey, string> = {
  control1: "C1",
  control2: "C2",
  end: "E",
  start: "S",
};

function cloneThreadBundles(bundles: ThreadBundleConfig[]): ThreadBundleConfig[] {
  return bundles.map((bundle) => ({
    ...bundle,
    control1: { ...bundle.control1 },
    control2: { ...bundle.control2 },
    end: { ...bundle.end },
    start: { ...bundle.start },
  }));
}

function cubicBezierPoint(
  start: Point,
  control1: Point,
  control2: Point,
  end: Point,
  t: number,
): Point {
  const inverse = 1 - t;
  return {
    x:
      inverse ** 3 * start.x +
      3 * inverse ** 2 * t * control1.x +
      3 * inverse * t ** 2 * control2.x +
      t ** 3 * end.x,
    y:
      inverse ** 3 * start.y +
      3 * inverse ** 2 * t * control1.y +
      3 * inverse * t ** 2 * control2.y +
      t ** 3 * end.y,
  };
}

function createHeroPoint(heroRect: HeroRectMetrics, xFactor: number, yFactor: number): Point {
  return {
    x: heroRect.left + heroRect.width * xFactor,
    y: heroRect.top + heroRect.height * yFactor,
  };
}

function resolveHeroPoint(point: BlobPoint, heroRect: HeroRectMetrics): Point {
  return createHeroPoint(heroRect, point.x, point.y);
}

function resolveBundleScenePoints(bundle: ThreadBundleConfig, heroRect: HeroRectMetrics): BundleScenePoints {
  return {
    control1: resolveHeroPoint(bundle.control1, heroRect),
    control2: resolveHeroPoint(bundle.control2, heroRect),
    end: resolveHeroPoint(bundle.end, heroRect),
    start: resolveHeroPoint(bundle.start, heroRect),
  };
}

function buildBundlePath(points: BundleScenePoints): string {
  return `M ${points.start.x} ${points.start.y} C ${points.control1.x} ${points.control1.y}, ${points.control2.x} ${points.control2.y}, ${points.end.x} ${points.end.y}`;
}

function toThreadFactorPoint(point: Point, heroRect: HeroRectMetrics): BlobPoint {
  return {
    x: clamp((point.x - heroRect.left) / heroRect.width, -0.6, 1.4),
    y: clamp((point.y - heroRect.top) / heroRect.height, -0.2, 1.4),
  };
}

function drawThreadBundle(
  context: CanvasRenderingContext2D,
  bundle: ThreadBundleConfig,
  heroRect: HeroRectMetrics,
  time: number,
) {
  const points = resolveBundleScenePoints(bundle, heroRect);
  const { control1, control2, end, start } = points;
  const dx = end.x - start.x;
  const dy = end.y - start.y;
  const distance = Math.hypot(dx, dy);
  if (distance < 24) {
    return;
  }

  const normal = { x: -dy / distance, y: dx / distance };
  const lift = Math.min(96, distance * 0.18);

  context.save();
  context.beginPath();
  context.moveTo(start.x, start.y);
  context.bezierCurveTo(control1.x, control1.y, control2.x, control2.y, end.x, end.y);
  context.strokeStyle = `rgba(255,255,255,${bundle.glowAlpha})`;
  context.lineWidth = bundle.glowWidth;
  context.shadowBlur = 18;
  context.shadowColor = `rgba(255,255,255,${bundle.glowAlpha * 0.8})`;
  context.stroke();
  context.restore();

  for (let strandIndex = 0; strandIndex < bundle.strandCount; strandIndex += 1) {
    const strandSeed = bundle.seed * 0.53 + strandIndex * 0.61;
    const offset = (strandIndex - (bundle.strandCount - 1) / 2) * 4.2;
    const drift = Math.sin(time * 0.0009 + strandSeed) * 2.8;
    const strandControl1 = {
      x: control1.x + normal.x * (offset + drift),
      y: control1.y + normal.y * (offset + drift) - lift * 0.02,
    };
    const strandControl2 = {
      x: control2.x + normal.x * (offset * 0.66 - drift * 0.24),
      y: control2.y + normal.y * (offset * 0.66 - drift * 0.24) + lift * 0.02,
    };

    context.beginPath();
    context.moveTo(start.x, start.y);
    context.bezierCurveTo(strandControl1.x, strandControl1.y, strandControl2.x, strandControl2.y, end.x, end.y);
    context.strokeStyle = "rgba(255,255,255,0.08)";
    context.lineWidth = 2.3;
    context.stroke();

    context.beginPath();
    context.moveTo(start.x, start.y);
    context.bezierCurveTo(strandControl1.x, strandControl1.y, strandControl2.x, strandControl2.y, end.x, end.y);
    context.strokeStyle = `rgba(255,255,255,${(0.12 + strandIndex * 0.03) * bundle.groupAlpha})`;
    context.lineWidth = 0.75;
    context.stroke();

    const sparklePoint = cubicBezierPoint(
      start,
      strandControl1,
      strandControl2,
      end,
      (time * 0.00008 + strandSeed * 0.12) % 1,
    );
    context.save();
    context.fillStyle = "rgba(255,255,255,0.88)";
    context.shadowBlur = 12;
    context.shadowColor = "rgba(255,255,255,0.72)";
    context.beginPath();
    context.arc(sparklePoint.x, sparklePoint.y, 1.4, 0, Math.PI * 2);
    context.fill();
    context.restore();
  }
}

function drawInteriorVeils(context: CanvasRenderingContext2D, heroRect: HeroRectMetrics) {
  const veilPaths = [
    {
      control1: createHeroPoint(heroRect, 0.12, 0.86),
      control2: createHeroPoint(heroRect, 0.22, 0.8),
      end: createHeroPoint(heroRect, 0.34, 0.82),
      start: createHeroPoint(heroRect, -0.02, 0.88),
    },
    {
      control1: createHeroPoint(heroRect, 0.16, 0.92),
      control2: createHeroPoint(heroRect, 0.3, 0.86),
      end: createHeroPoint(heroRect, 0.46, 0.88),
      start: createHeroPoint(heroRect, 0.04, 0.95),
    },
  ];

  veilPaths.forEach((veil) => {
    context.save();
    context.beginPath();
    context.moveTo(veil.start.x, veil.start.y);
    context.bezierCurveTo(
      veil.control1.x,
      veil.control1.y,
      veil.control2.x,
      veil.control2.y,
      veil.end.x,
      veil.end.y,
    );
    context.strokeStyle = "rgba(255,255,255,0.055)";
    context.lineWidth = 10;
    context.shadowBlur = 26;
    context.shadowColor = "rgba(186,194,255,0.12)";
    context.stroke();
    context.restore();
  });
}

function OrbitTokenChip({
  token,
  className,
  delay,
}: {
  token: SpotlightToken;
  className: string;
  delay: number;
}) {
  return (
    <motion.div
      animate={{
        opacity: [0.72, 1, 0.76],
        rotate: token.kind === "room" ? [-6, 4, -6] : [0, 6, 0],
        y: [0, -8, 0],
      }}
      className={cn(
        "pointer-events-none absolute z-[2] flex items-center justify-center rounded-[18px] border border-white/28 text-[10px] font-semibold tracking-[0.12em] text-white/88 shadow-[0_10px_26px_rgba(7,10,22,0.16)] backdrop-blur-sm",
        token.kind === "agent" ? "h-13 w-13 rounded-full" : "h-14 w-14",
        className,
      )}
      style={{
        background: `radial-gradient(circle at 30% 24%, rgba(255,255,255,0.4) 0%, transparent 38%), linear-gradient(180deg, ${token.swatch.fill} 0%, rgba(255,255,255,0.2) 100%)`,
      }}
      transition={{
        delay,
        duration: 6.4,
        ease: "easeInOut",
        repeat: Infinity,
        repeatType: "mirror",
      }}
    >
      {token.label}
    </motion.div>
  );
}

export function HeroThreadField({ sceneRef, tokens }: HeroThreadFieldProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const debugSvgRef = useRef<SVGSVGElement | null>(null);
  const activeHandleRef = useRef<ThreadHandleState>(null);
  const [bundles, setBundles] = useState<ThreadBundleConfig[]>(() => cloneThreadBundles(DEFAULT_THREAD_BUNDLES));
  const [sceneMetrics, setSceneMetrics] = useState<SceneMetrics>({
    height: 0,
    heroRect: null,
    width: 0,
  });
  const debugEnabled = useBlobDebugEnabled();
  const { setTarget, target } = useBlobDebugTarget();
  const debugActive = debugEnabled && target === "thread";
  const debugSvgRect = useDebugSvgRect(debugActive, debugSvgRef);
  const orbitTokens = useMemo(() => tokens.slice(0, ORBIT_TOKEN_LAYOUTS.length), [tokens]);
  const flattenedPoints = useMemo(
    () => bundles.flatMap((bundle) => THREAD_POINT_KEYS.map((pointKey) => bundle[pointKey])),
    [bundles],
  );
  const sceneBundlePoints = useMemo(() => {
    if (!sceneMetrics.heroRect) {
      return [];
    }

    return bundles.map((bundle) => resolveBundleScenePoints(bundle, sceneMetrics.heroRect!));
  }, [bundles, sceneMetrics.heroRect]);
  const threadHandleEntries = useMemo(
    () =>
      sceneBundlePoints.flatMap((bundlePoints, bundleIndex) =>
        THREAD_POINT_KEYS.map((pointKey) => ({
          bundleIndex,
          point: bundlePoints[pointKey],
          pointKey,
        })),
      ),
    [sceneBundlePoints],
  );

  useEffect(() => {
    const persisted = parseThreadBundles(window.localStorage.getItem(THREAD_STORAGE_KEY));
    if (persisted) {
      setBundles(cloneThreadBundles(persisted));
    }
  }, []);

  useEffect(() => {
    window.localStorage.setItem(THREAD_STORAGE_KEY, serializeThreadBundles(bundles));
  }, [bundles]);

  useEffect(() => {
    const scene = sceneRef.current;
    if (!scene) {
      return;
    }

    const updateMetrics = () => {
      const sceneRect = scene.getBoundingClientRect();
      const heroShell = scene.querySelector<HTMLElement>("[data-thread-anchor-group='hero-shell']");
      const heroRect = heroShell?.getBoundingClientRect();
      setSceneMetrics({
        height: sceneRect.height,
        heroRect:
          heroRect
            ? {
              height: heroRect.height,
              left: heroRect.left - sceneRect.left,
              top: heroRect.top - sceneRect.top,
              width: heroRect.width,
            }
            : null,
        width: sceneRect.width,
      });
    };

    updateMetrics();
    const resizeObserver = new ResizeObserver(updateMetrics);
    resizeObserver.observe(scene);

    const heroShell = scene.querySelector<HTMLElement>("[data-thread-anchor-group='hero-shell']");
    if (heroShell) {
      resizeObserver.observe(heroShell);
    }

    window.addEventListener("resize", updateMetrics);
    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", updateMetrics);
    };
  }, [sceneRef]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const scene = sceneRef.current;
    if (!canvas || !scene) {
      return;
    }

    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }

    let frameId = 0;
    let devicePixelRatio = window.devicePixelRatio || 1;

    const resizeCanvas = () => {
      const rect = scene.getBoundingClientRect();
      devicePixelRatio = window.devicePixelRatio || 1;
      canvas.width = Math.max(1, Math.round(rect.width * devicePixelRatio));
      canvas.height = Math.max(1, Math.round(rect.height * devicePixelRatio));
      canvas.style.width = `${rect.width}px`;
      canvas.style.height = `${rect.height}px`;
      context.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
    };

    const resizeObserver = new ResizeObserver(() => {
      resizeCanvas();
    });
    resizeObserver.observe(scene);
    resizeCanvas();

    const render = (time: number) => {
      const currentScene = sceneRef.current;
      if (!currentScene) {
        return;
      }

      const sceneRect = currentScene.getBoundingClientRect();
      context.clearRect(0, 0, sceneRect.width, sceneRect.height);
      context.save();
      context.globalCompositeOperation = "screen";

      const heroShell = currentScene.querySelector<HTMLElement>("[data-thread-anchor-group='hero-shell']");
      if (heroShell) {
        const heroRect = heroShell.getBoundingClientRect();
        const heroMetrics = {
          height: heroRect.height,
          left: heroRect.left - sceneRect.left,
          top: heroRect.top - sceneRect.top,
          width: heroRect.width,
        };

        bundles.forEach((bundle) => {
          drawThreadBundle(context, bundle, heroMetrics, time);
        });
        drawInteriorVeils(context, heroMetrics);
      }

      context.restore();
      frameId = window.requestAnimationFrame(render);
    };

    frameId = window.requestAnimationFrame(render);
    return () => {
      window.cancelAnimationFrame(frameId);
      resizeObserver.disconnect();
    };
  }, [bundles, sceneRef]);

  const toSceneCoordinates = useCallback((clientX: number, clientY: number): Point | null => {
    const svgElement = debugSvgRef.current;
    if (!svgElement || sceneMetrics.width === 0 || sceneMetrics.height === 0) {
      return null;
    }

    const rect = svgElement.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) {
      return null;
    }

    return {
      x: ((clientX - rect.left) / rect.width) * sceneMetrics.width,
      y: ((clientY - rect.top) / rect.height) * sceneMetrics.height,
    };
  }, [sceneMetrics.height, sceneMetrics.width]);

  useEffect(() => {
    if (!debugActive || !sceneMetrics.heroRect) {
      return;
    }

    const heroRect = sceneMetrics.heroRect;

    const handlePointerMove = (event: PointerEvent) => {
      const activeHandle = activeHandleRef.current;
      if (!activeHandle) {
        return;
      }

      const scenePoint = toSceneCoordinates(event.clientX, event.clientY);
      if (!scenePoint) {
        return;
      }

      const factorPoint = toThreadFactorPoint(scenePoint, heroRect);
      setBundles((current) =>
        current.map((bundle, index) =>
          index === activeHandle.bundleIndex
            ? {
              ...bundle,
              [activeHandle.pointKey]: factorPoint,
            }
            : bundle,
        ),
      );
    };

    const handlePointerUp = (event: PointerEvent) => {
      if (activeHandleRef.current && activeHandleRef.current.pointerId !== event.pointerId) {
        return;
      }

      activeHandleRef.current = null;
      document.body.style.cursor = "default";
    };

    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    window.addEventListener("pointercancel", handlePointerUp);
    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.removeEventListener("pointercancel", handlePointerUp);
      document.body.style.cursor = "default";
    };
  }, [debugActive, sceneMetrics.heroRect, toSceneCoordinates]);

  const handlePointPointerDown = useCallback(
    (index: number) => (event: ReactPointerEvent<Element>) => {
      if (!debugActive) {
        return;
      }

      const handleEntry = threadHandleEntries[index];
      if (!handleEntry) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      document.body.style.cursor = "grabbing";
      activeHandleRef.current = {
        bundleIndex: handleEntry.bundleIndex,
        pointKey: handleEntry.pointKey,
        pointerId: event.pointerId,
      };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [debugActive, threadHandleEntries],
  );

  const handlePointPointerUp = useCallback((event: ReactPointerEvent<Element>) => {
    if (!activeHandleRef.current || activeHandleRef.current.pointerId !== event.pointerId) {
      return;
    }

    activeHandleRef.current = null;
    document.body.style.cursor = "default";
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const handleReset = useCallback(() => {
    window.localStorage.removeItem(THREAD_STORAGE_KEY);
    setBundles(cloneThreadBundles(DEFAULT_THREAD_BUNDLES));
  }, []);

  return (
    <div className="pointer-events-none absolute inset-0 z-0 overflow-visible">
      <canvas
        ref={canvasRef}
        aria-hidden="true"
        className="absolute inset-0 h-full w-full"
      />

      {orbitTokens.map((token, index) => {
        const layout = ORBIT_TOKEN_LAYOUTS[index];
        if (!layout) {
          return null;
        }

        return (
          <OrbitTokenChip
            key={token.key}
            className={layout.className}
            delay={layout.delay}
            token={token}
          />
        );
      })}

      {debugActive && sceneMetrics.heroRect && sceneMetrics.width > 0 && sceneMetrics.height > 0 && (
        <>
          <svg
            ref={debugSvgRef}
            aria-hidden="true"
            className="pointer-events-none absolute inset-0 z-20 h-full w-full"
            preserveAspectRatio="none"
            viewBox={`0 0 ${sceneMetrics.width} ${sceneMetrics.height}`}
          >
            {sceneBundlePoints.map((bundlePoints, bundleIndex) => (
              <g key={`thread-bundle-${bundleIndex}`}>
                <path
                  d={buildBundlePath(bundlePoints)}
                  fill="none"
                  stroke="rgba(118,231,206,0.72)"
                  strokeDasharray="8 6"
                  strokeWidth="1.2"
                />
                <line
                  stroke="rgba(255,255,255,0.22)"
                  strokeDasharray="4 4"
                  strokeWidth="1"
                  x1={bundlePoints.start.x}
                  x2={bundlePoints.control1.x}
                  y1={bundlePoints.start.y}
                  y2={bundlePoints.control1.y}
                />
                <line
                  stroke="rgba(255,255,255,0.22)"
                  strokeDasharray="4 4"
                  strokeWidth="1"
                  x1={bundlePoints.control2.x}
                  x2={bundlePoints.end.x}
                  y1={bundlePoints.control2.y}
                  y2={bundlePoints.end.y}
                />

                {THREAD_POINT_KEYS.map((pointKey) => {
                  const point = bundlePoints[pointKey];
                  return (
                    <g key={`thread-point-${bundleIndex}-${pointKey}`}>
                      <circle
                        cx={point.x}
                        cy={point.y}
                        fill="rgba(255,255,255,0.94)"
                        pointerEvents="none"
                        r="7"
                        stroke="rgba(118,231,206,0.92)"
                        strokeWidth="2.6"
                      />
                      <text
                        fill="rgba(255,255,255,0.72)"
                        fontSize="11"
                        pointerEvents="none"
                        textAnchor="middle"
                        x={point.x}
                        y={point.y - 14}
                      >
                        {`${bundleIndex + 1}${THREAD_POINT_LABELS[pointKey]}`}
                      </text>
                    </g>
                  );
                })}
              </g>
            ))}
          </svg>

          <DebugPortalHandles
            debugEnabled={debugActive}
            handleClassName="border-white/20 bg-white/8 shadow-[0_0_0_1px_rgba(118,231,206,0.18)]"
            onPointPointerDown={handlePointPointerDown}
            onPointPointerUp={handlePointPointerUp}
            points={threadHandleEntries.map((entry) => entry.point)}
            svgRect={debugSvgRect}
            viewBoxHeight={sceneMetrics.height}
            viewBoxWidth={sceneMetrics.width}
          />

          <BlobDebugPanel
            countLabel="当前控制点"
            currentTarget={target}
            description="直接拖拽起点、控制点和落点，调整丝线走向。"
            onCopy={async () => {
              await navigator.clipboard.writeText(serializeThreadBundles(bundles));
            }}
            onReset={handleReset}
            panelClassName="bottom-4 left-4"
            points={flattenedPoints}
            setTarget={setTarget}
            target={target}
            title="Thread Bundles"
          />
        </>
      )}
    </div>
  );
}
