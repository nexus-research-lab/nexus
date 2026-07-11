import {
  type CSSProperties,
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";

import { cn } from "@/shared/ui/class-name";

import { supportsTrueLiquidGlass } from "./liquid-glass-engine";

interface GlassMagnifierProps {
  children?: ReactNode;
  className?: string;
  contentClassName?: string;
  height?: number;
  underlay?: ReactNode;
  width?: number;
}

const BASE_LENS_WIDTH = 210;
const BASE_LENS_HEIGHT = 150;
const BASE_LENS_RADIUS = 75;
const MAGNIFYING_SCALE = 24;
const DISPLACEMENT_SCALE = 98.24713343067756;
const SATURATION_VALUE = 9;
const SPECULAR_FADE_SLOPE = 0.5;
const HOVER_LOOP_DURATION_MS = 1560;
const SETTLE_DURATION_MS = 260;
const MAGNIFYING_MAP_URL = "/liquid-glass/magnifier-magnifying-map.png";
const DISPLACEMENT_MAP_URL = "/liquid-glass/magnifier-displacement-map.png";
const SPECULAR_MAP_URL = "/liquid-glass/magnifier-specular-map.png";

function buildGlassSurfaceStyle(filterId: string | null): CSSProperties {
  return {
    borderRadius: `${BASE_LENS_RADIUS}px`,
    boxShadow: "rgba(0, 0, 0, 0.16) 0px 4px 9px, rgba(0, 0, 0, 0.2) 0px 2px 24px inset, rgba(255, 255, 255, 0.2) 0px -2px 24px inset",
    transform: "translateZ(0px)",
    backdropFilter: filterId ? `url(#${filterId})` : "blur(16px)",
    WebkitBackdropFilter: filterId ? `url(#${filterId})` : "blur(16px)",
    backgroundColor: filterId ? "rgba(255, 255, 255, 0.01)" : "color-mix(in srgb, var(--surface-panel-background) 72%, transparent)",
  };
}

export function GlassMagnifier({
  children,
  className: className,
  contentClassName: contentClassName,
  height = 36,
  underlay,
  width = 58,
}: GlassMagnifierProps) {
  const rawFilterId = useId();
  const filterId = `glass-magnifier-${rawFilterId.replace(/:/g, "")}`;
  const [canUseTrueGlass, setCanUseTrueGlass] = useState<boolean>(() => supportsTrueLiquidGlass());
  const rootRef = useRef<HTMLDivElement | null>(null);
  const shellRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const rootAnimationRef = useRef<Animation | null>(null);
  const contentAnimationRef = useRef<Animation | null>(null);
  const baseScaleX = width / BASE_LENS_WIDTH;
  const baseScaleY = height / BASE_LENS_HEIGHT;
  const idleTransform = `scale(${baseScaleX}, ${baseScaleY})`;

  const buildRootTransform = useCallback((xMultiplier: number, yMultiplier: number) => {
    return `translateZ(0px) scale(${xMultiplier}, ${yMultiplier})`;
  }, []);

  const buildContentTransform = useCallback((xMultiplier: number, yMultiplier: number, yOffset: number) => {
    return `translate3d(0px, ${yOffset}px, 0px) scale(${xMultiplier}, ${yMultiplier})`;
  }, []);

  const readCurrentTransform = useCallback((element: HTMLDivElement | null, fallback: string) => {
    if (!element || typeof window === "undefined") {
      return fallback;
    }
    const computedTransform = window.getComputedStyle(element).transform;
    return computedTransform && computedTransform !== "none" ? computedTransform : fallback;
  }, []);

  const animateLayer = useCallback((
    element: HTMLDivElement | null,
    animationRef: React.MutableRefObject<Animation | null>,
    keyframes: Keyframe[],
    duration: number,
    easing: string,
    iterations = 1,
  ) => {
    if (!element) {
      return;
    }

    animationRef.current?.cancel();

    if (typeof window === "undefined" || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      const finalTransform = keyframes[keyframes.length - 1]?.transform;
      if (typeof finalTransform === "string") {
        element.style.transform = finalTransform;
      }
      return;
    }

    const animation = element.animate(keyframes, {
      duration,
      easing,
      fill: "forwards",
      iterations,
    });
    animationRef.current = animation;
  }, []);

  const animateHoverWave = useCallback(() => {
    animateLayer(
      rootRef.current,
      rootAnimationRef,
      [
        { transform: buildRootTransform(1, 1), offset: 0 },
        { transform: buildRootTransform(0.958, 1.104), offset: 0.14 },
        { transform: buildRootTransform(1.024, 0.972), offset: 0.3 },
        { transform: buildRootTransform(0.976, 1.068), offset: 0.48 },
        { transform: buildRootTransform(1.016, 0.986), offset: 0.66 },
        { transform: buildRootTransform(0.988, 1.034), offset: 0.82 },
        { transform: buildRootTransform(1, 1), offset: 1 },
      ],
      HOVER_LOOP_DURATION_MS,
      "ease-in-out",
      Number.POSITIVE_INFINITY,
    );

    animateLayer(
      contentRef.current,
      contentAnimationRef,
      [
        { transform: buildContentTransform(1, 1, 0), offset: 0 },
        { transform: buildContentTransform(1.028, 0.948, -0.72), offset: 0.16 },
        { transform: buildContentTransform(0.98, 1.032, 0.16), offset: 0.34 },
        { transform: buildContentTransform(1.016, 0.972, -0.42), offset: 0.54 },
        { transform: buildContentTransform(0.99, 1.018, 0.08), offset: 0.74 },
        { transform: buildContentTransform(1.008, 0.988, -0.18), offset: 0.88 },
        { transform: buildContentTransform(1, 1, 0), offset: 1 },
      ],
      HOVER_LOOP_DURATION_MS,
      "ease-in-out",
      Number.POSITIVE_INFINITY,
    );
  }, [
    animateLayer,
    buildContentTransform,
    buildRootTransform,
  ]);

  const settleHoverWave = useCallback(() => {
    const rootCurrentTransform = readCurrentTransform(rootRef.current, buildRootTransform(1, 1));
    const contentCurrentTransform = readCurrentTransform(contentRef.current, buildContentTransform(1, 1, 0));

    animateLayer(
      rootRef.current,
      rootAnimationRef,
      [
        { transform: rootCurrentTransform, offset: 0 },
        { transform: buildRootTransform(1.006, 0.994), offset: 0.58 },
        { transform: buildRootTransform(1, 1), offset: 1 },
      ],
      SETTLE_DURATION_MS,
      "cubic-bezier(0.22, 1, 0.36, 1)",
    );

    animateLayer(
      contentRef.current,
      contentAnimationRef,
      [
        { transform: contentCurrentTransform, offset: 0 },
        { transform: buildContentTransform(1.004, 0.996, -0.06), offset: 0.6 },
        { transform: buildContentTransform(1, 1, 0), offset: 1 },
      ],
      SETTLE_DURATION_MS,
      "cubic-bezier(0.22, 1, 0.36, 1)",
    );
  }, [
    animateLayer,
    buildContentTransform,
    buildRootTransform,
    readCurrentTransform,
  ]);

  const handleHoverStart = useCallback(() => {
    animateHoverWave();
  }, [animateHoverWave]);

  const handleHoverEnd = useCallback(() => {
    settleHoverWave();
  }, [settleHoverWave]);

  useEffect(() => {
    setCanUseTrueGlass(supportsTrueLiquidGlass());
  }, []);

  useEffect(() => {
    const rootElement = rootRef.current;
    const shellElement = shellRef.current;
    const contentElement = contentRef.current;
    if (rootElement) {
      rootElement.style.transform = buildRootTransform(1, 1);
    }
    if (!shellElement) {
      return;
    }
    shellElement.style.transform = idleTransform;
    if (contentElement) {
      contentElement.style.transform = buildContentTransform(1, 1, 0);
    }
  }, [buildContentTransform, buildRootTransform, idleTransform]);

  useEffect(() => {
    const rootAnimation = rootAnimationRef.current;
    const contentAnimation = contentAnimationRef.current;
    return () => {
      rootAnimation?.cancel();
      contentAnimation?.cancel();
    };
  }, []);

  return (
    <div
      className={cn("relative isolate shrink-0 cursor-grab select-none active:cursor-grabbing", className)}
      onPointerCancel={handleHoverEnd}
      onPointerEnter={handleHoverStart}
      onPointerLeave={handleHoverEnd}
      ref={rootRef}
      style={{
        height: `${height}px`,
        touchAction: "none",
        transformOrigin: "50% 50%",
        userSelect: "none",
        willChange: "transform",
        width: `${width}px`,
      }}
    >
      {underlay ? (
        <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-[999px]">
          {underlay}
        </div>
      ) : null}

      {canUseTrueGlass ? (
        <svg
          aria-hidden="true"
          className="hidden"
          colorInterpolationFilters="sRGB"
          focusable="false"
        >
          <defs>
            <filter id={filterId}>
              <feImage
                href={MAGNIFYING_MAP_URL}
                result="magnifying_displacement_map"
                x={0}
                y={0}
                width={BASE_LENS_WIDTH}
                height={BASE_LENS_HEIGHT}
              />
              <feDisplacementMap
                in="SourceGraphic"
                in2="magnifying_displacement_map"
                result="magnified_source"
                scale={MAGNIFYING_SCALE}
                xChannelSelector="R"
                yChannelSelector="G"
              />
              <feGaussianBlur
                in="magnified_source"
                result="blurred_source"
                stdDeviation={0}
              />
              <feImage
                href={DISPLACEMENT_MAP_URL}
                result="displacement_map"
                x={0}
                y={0}
                width={BASE_LENS_WIDTH}
                height={BASE_LENS_HEIGHT}
              />
              <feDisplacementMap
                in="blurred_source"
                in2="displacement_map"
                result="displaced"
                scale={DISPLACEMENT_SCALE}
                xChannelSelector="R"
                yChannelSelector="G"
              />
              <feColorMatrix
                in="displaced"
                result="displaced_saturated"
                type="saturate"
                values={String(SATURATION_VALUE)}
              />
              <feImage
                href={SPECULAR_MAP_URL}
                result="specular_layer"
                x={0}
                y={0}
                width={BASE_LENS_WIDTH}
                height={BASE_LENS_HEIGHT}
              />
              <feComposite
                in="displaced_saturated"
                in2="specular_layer"
                operator="in"
                result="specular_saturated"
              />
              <feComponentTransfer
                in="specular_layer"
                result="specular_faded"
              >
                <feFuncA type="linear" slope={SPECULAR_FADE_SLOPE} />
              </feComponentTransfer>
              <feBlend
                in="specular_saturated"
                in2="displaced"
                mode="normal"
                result="with_saturation"
              />
              <feBlend
                in="specular_faded"
                in2="with_saturation"
                mode="normal"
              />
            </filter>
          </defs>
        </svg>
      ) : null}

      <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-[999px]">
        <div
          className="absolute left-0 top-0 origin-top-left ring-1 ring-black/10 dark:ring-white/10"
          ref={shellRef}
          style={{
            ...buildGlassSurfaceStyle(canUseTrueGlass ? filterId : null),
            height: `${BASE_LENS_HEIGHT}px`,
            transform: idleTransform,
            width: `${BASE_LENS_WIDTH}px`,
            willChange: "transform",
          }}
        />
      </div>

      {children ? (
        <div
          className={cn(
            "pointer-events-none absolute inset-0 z-10 flex items-center justify-center",
            contentClassName,
          )}
          ref={contentRef}
          style={{
            transform: buildContentTransform(1, 1, 0),
            transformOrigin: "50% 50%",
            willChange: "transform",
          }}
        >
          {children}
        </div>
      ) : null}
    </div>
  );
}
