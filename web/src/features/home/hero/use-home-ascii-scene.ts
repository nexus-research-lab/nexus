"use client";

import { type RefObject, useEffect } from "react";

import { createHomeAsciiScene } from "./home-ascii-scene";

interface UseHomeAsciiSceneOptions {
  canvasRef: RefObject<HTMLCanvasElement | null>;
  enabled: boolean;
  sectionRef: RefObject<HTMLElement | null>;
  themeKey: string;
}

export function useHomeAsciiScene({
  canvasRef,
  enabled,
  sectionRef,
  themeKey,
}: UseHomeAsciiSceneOptions): void {
  useEffect(() => {
    const section = sectionRef.current;
    const canvas = canvasRef.current;
    if (!enabled || !section || !canvas) {
      return;
    }

    const scene = createHomeAsciiScene(section, canvas);
    scene?.start();
    return () => scene?.dispose();
  }, [canvasRef, enabled, sectionRef, themeKey]);
}
