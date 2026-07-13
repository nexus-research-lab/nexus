import {
  type MutableRefObject,
  type RefObject,
  useCallback,
  useEffect,
  useRef,
} from "react";

interface GlassMagnifierAnimationOptions {
  height: number;
  width: number;
}

interface GlassMagnifierAnimation {
  contentRef: RefObject<HTMLDivElement | null>;
  contentTransform: string;
  idleTransform: string;
  onHoverEnd: () => void;
  onHoverStart: () => void;
  rootRef: RefObject<HTMLDivElement | null>;
  rootTransform: string;
  shellRef: RefObject<HTMLDivElement | null>;
  sourceSize: {
    height: number;
    width: number;
  };
}

const SOURCE_SIZE = {
  height: 150,
  width: 210,
} as const;
const HOVER_LOOP_DURATION_MS = 1560;
const SETTLE_DURATION_MS = 260;
const HOVER_EASING = "ease-in-out";
const SETTLE_EASING = "cubic-bezier(0.22, 1, 0.36, 1)";

function buildRootTransform(xMultiplier: number, yMultiplier: number): string {
  return `translateZ(0px) scale(${xMultiplier}, ${yMultiplier})`;
}

function buildContentTransform(
  xMultiplier: number,
  yMultiplier: number,
  yOffset: number,
): string {
  return `translate3d(0px, ${yOffset}px, 0px) scale(${xMultiplier}, ${yMultiplier})`;
}

function readCurrentTransform(element: HTMLDivElement | null, fallback: string): string {
  if (!element || typeof window === "undefined") {
    return fallback;
  }
  const transform = window.getComputedStyle(element).transform;
  return transform && transform !== "none" ? transform : fallback;
}

function animateLayer(
  element: HTMLDivElement | null,
  animationRef: MutableRefObject<Animation | null>,
  keyframes: Keyframe[],
  options: KeyframeAnimationOptions,
): void {
  if (!element) {
    return;
  }

  animationRef.current?.cancel();
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    const finalTransform = keyframes.at(-1)?.transform;
    if (typeof finalTransform === "string") {
      element.style.transform = finalTransform;
    }
    return;
  }

  animationRef.current = element.animate(keyframes, {
    fill: "forwards",
    ...options,
  });
}

/** 放大镜动画集中持有 Web Animation 资源，视图只绑定引用和交互入口。 */
export function useGlassMagnifierAnimation({
  height,
  width,
}: GlassMagnifierAnimationOptions): GlassMagnifierAnimation {
  const rootRef = useRef<HTMLDivElement | null>(null);
  const shellRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const rootAnimationRef = useRef<Animation | null>(null);
  const contentAnimationRef = useRef<Animation | null>(null);
  const rootTransform = buildRootTransform(1, 1);
  const contentTransform = buildContentTransform(1, 1, 0);
  const idleTransform = `scale(${width / SOURCE_SIZE.width}, ${height / SOURCE_SIZE.height})`;

  const onHoverStart = useCallback(() => {
    animateLayer(
      rootRef.current,
      rootAnimationRef,
      [
        { transform: rootTransform, offset: 0 },
        { transform: buildRootTransform(0.958, 1.104), offset: 0.14 },
        { transform: buildRootTransform(1.024, 0.972), offset: 0.3 },
        { transform: buildRootTransform(0.976, 1.068), offset: 0.48 },
        { transform: buildRootTransform(1.016, 0.986), offset: 0.66 },
        { transform: buildRootTransform(0.988, 1.034), offset: 0.82 },
        { transform: rootTransform, offset: 1 },
      ],
      {
        duration: HOVER_LOOP_DURATION_MS,
        easing: HOVER_EASING,
        iterations: Number.POSITIVE_INFINITY,
      },
    );
    animateLayer(
      contentRef.current,
      contentAnimationRef,
      [
        { transform: contentTransform, offset: 0 },
        { transform: buildContentTransform(1.028, 0.948, -0.72), offset: 0.16 },
        { transform: buildContentTransform(0.98, 1.032, 0.16), offset: 0.34 },
        { transform: buildContentTransform(1.016, 0.972, -0.42), offset: 0.54 },
        { transform: buildContentTransform(0.99, 1.018, 0.08), offset: 0.74 },
        { transform: buildContentTransform(1.008, 0.988, -0.18), offset: 0.88 },
        { transform: contentTransform, offset: 1 },
      ],
      {
        duration: HOVER_LOOP_DURATION_MS,
        easing: HOVER_EASING,
        iterations: Number.POSITIVE_INFINITY,
      },
    );
  }, [contentTransform, rootTransform]);

  const onHoverEnd = useCallback(() => {
    animateLayer(
      rootRef.current,
      rootAnimationRef,
      [
        { transform: readCurrentTransform(rootRef.current, rootTransform), offset: 0 },
        { transform: buildRootTransform(1.006, 0.994), offset: 0.58 },
        { transform: rootTransform, offset: 1 },
      ],
      { duration: SETTLE_DURATION_MS, easing: SETTLE_EASING },
    );
    animateLayer(
      contentRef.current,
      contentAnimationRef,
      [
        {
          transform: readCurrentTransform(contentRef.current, contentTransform),
          offset: 0,
        },
        { transform: buildContentTransform(1.004, 0.996, -0.06), offset: 0.6 },
        { transform: contentTransform, offset: 1 },
      ],
      { duration: SETTLE_DURATION_MS, easing: SETTLE_EASING },
    );
  }, [contentTransform, rootTransform]);

  useEffect(() => () => {
    rootAnimationRef.current?.cancel();
    contentAnimationRef.current?.cancel();
  }, []);

  return {
    contentRef,
    contentTransform,
    idleTransform,
    onHoverEnd,
    onHoverStart,
    rootRef,
    rootTransform,
    shellRef,
    sourceSize: SOURCE_SIZE,
  };
}
