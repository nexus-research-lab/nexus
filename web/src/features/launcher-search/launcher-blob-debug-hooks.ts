"use client";

import { type PointerEvent as ReactPointerEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  clamp,
  createClosedSplinePath,
  findNearestSegmentIndex,
  parsePoints,
} from "@/features/launcher-search/launcher-blob-shape";
import { BlobPoint } from "@/types/launcher";

export type BlobDebugTarget = "hero" | "input" | "panel";

interface EditableShapeOptions {
  defaultPoints: BlobPoint[];
  storageKey: string;
  viewBoxWidth: number;
  viewBoxHeight: number;
}

const DEBUG_TARGET_STORAGE_KEY = "nexus-home-blob-debug-target";

function isBlobDebugTarget(value: string | null): value is BlobDebugTarget {
  return value === "hero" || value === "input" || value === "panel";
}

export function useBlobDebugEnabled(): boolean {
  const [enabled, setEnabled] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    setEnabled(params.get("blobDebug") === "1");
  }, []);

  return enabled;
}

export function useBlobDebugTarget() {
  const [target, setTargetState] = useState<BlobDebugTarget>("hero");

  useEffect(() => {
    const persisted = window.localStorage.getItem(DEBUG_TARGET_STORAGE_KEY);
    if (isBlobDebugTarget(persisted)) {
      setTargetState(persisted);
    }

    const handleTargetChange = (event: Event) => {
      const nextTarget = (event as CustomEvent<BlobDebugTarget>).detail;
      if (isBlobDebugTarget(nextTarget)) {
        setTargetState(nextTarget);
      }
    };

    window.addEventListener("nexus-blob-debug-target-change", handleTargetChange);
    return () => {
      window.removeEventListener("nexus-blob-debug-target-change", handleTargetChange);
    };
  }, []);

  const setTarget = useCallback((nextTarget: BlobDebugTarget) => {
    window.localStorage.setItem(DEBUG_TARGET_STORAGE_KEY, nextTarget);
    setTargetState(nextTarget);
    window.dispatchEvent(new CustomEvent("nexus-blob-debug-target-change", { detail: nextTarget }));
  }, []);

  return { setTarget, target };
}

export function useEditableShape({
  defaultPoints,
  storageKey,
  viewBoxHeight,
  viewBoxWidth,
}: EditableShapeOptions) {
  const svgRef = useRef<SVGSVGElement | null>(null);
  const activePointIndexRef = useRef<number | null>(null);
  const activePointerIdRef = useRef<number | null>(null);
  const [points, setPoints] = useState<BlobPoint[]>(defaultPoints);
  const debugEnabled = useBlobDebugEnabled();

  useEffect(() => {
    const persisted = parsePoints(localStorage.getItem(storageKey), viewBoxWidth, viewBoxHeight);
    if (persisted) {
      setPoints(persisted);
    }
  }, [storageKey, viewBoxHeight, viewBoxWidth]);

  useEffect(() => {
    localStorage.setItem(storageKey, JSON.stringify(points));
  }, [points, storageKey]);

  const path = useMemo(() => createClosedSplinePath(points), [points]);

  const toSvgCoordinates = useCallback((clientX: number, clientY: number) => {
    const svg = svgRef.current;
    if (!svg) {
      return null;
    }

    const rect = svg.getBoundingClientRect();
    if (rect.width === 0 || rect.height === 0) {
      return null;
    }

    return {
      x: ((clientX - rect.left) / rect.width) * viewBoxWidth,
      y: ((clientY - rect.top) / rect.height) * viewBoxHeight,
    };
  }, [viewBoxHeight, viewBoxWidth]);

  useEffect(() => {
    if (!debugEnabled) {
      return;
    }

    const handlePointerMove = (event: PointerEvent) => {
      if (
        activePointIndexRef.current === null ||
        activePointerIdRef.current === null ||
        activePointerIdRef.current !== event.pointerId
      ) {
        return;
      }

      const coordinates = toSvgCoordinates(event.clientX, event.clientY);
      if (!coordinates) {
        return;
      }

      setPoints((current) =>
        current.map((point, index) =>
          index === activePointIndexRef.current
            ? {
              x: clamp(coordinates.x, 16, viewBoxWidth - 16),
              y: clamp(coordinates.y, 16, viewBoxHeight - 16),
            }
            : point,
        ),
      );
    };

    const handlePointerUp = (event: PointerEvent) => {
      if (activePointerIdRef.current !== null && activePointerIdRef.current !== event.pointerId) {
        return;
      }

      activePointIndexRef.current = null;
      activePointerIdRef.current = null;
      document.body.style.cursor = "default";
    };

    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
    window.addEventListener("pointercancel", handlePointerUp);

    return () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.removeEventListener("pointercancel", handlePointerUp);
    };
  }, [debugEnabled, toSvgCoordinates, viewBoxHeight, viewBoxWidth]);

  const handlePointPointerDown = useCallback(
    (index: number) => (event: ReactPointerEvent<Element>) => {
      if (!debugEnabled) {
        return;
      }

      event.preventDefault();
      event.stopPropagation();
      document.body.style.cursor = "grabbing";
      activePointIndexRef.current = index;
      activePointerIdRef.current = event.pointerId;
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    [debugEnabled],
  );

  const handlePointPointerUp = useCallback((event: ReactPointerEvent<Element>) => {
    if (activePointerIdRef.current !== event.pointerId) {
      return;
    }

    activePointIndexRef.current = null;
    activePointerIdRef.current = null;
    document.body.style.cursor = "default";
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }, []);

  const handlePathDoubleClick = useCallback((event: ReactPointerEvent<SVGPathElement>) => {
    if (!debugEnabled) {
      return;
    }

    const coordinates = toSvgCoordinates(event.clientX, event.clientY);
    if (!coordinates) {
      return;
    }

    setPoints((current) => {
      const nearestSegmentIndex = findNearestSegmentIndex(current, coordinates);
      const next = [...current];
      next.splice(nearestSegmentIndex + 1, 0, {
        x: clamp(coordinates.x, 16, viewBoxWidth - 16),
        y: clamp(coordinates.y, 16, viewBoxHeight - 16),
      });
      return next;
    });
  }, [debugEnabled, toSvgCoordinates, viewBoxHeight, viewBoxWidth]);

  return {
    debugEnabled,
    handlePathDoubleClick,
    handlePointPointerDown,
    handlePointPointerUp,
    path,
    points,
    setPoints,
    svgRef,
  };
}
