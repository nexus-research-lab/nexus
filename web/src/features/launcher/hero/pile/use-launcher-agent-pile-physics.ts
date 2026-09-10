// INPUT: Token列表、DOM绑定与共享系统动效偏好。
// OUTPUT: 可销毁的物理场景或减少动效时的静态落位。
// POS: React到Matter的生命周期桥接，不持有业务状态。
"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import { usePrefersReducedMotion } from "@/shared/lib/react/use-prefers-reduced-motion";
import type { SpotlightToken } from "@/types/app/launcher";

import { createTokenConfig, LAUNCHER_PILE_WIDTH } from "./launcher-agent-pile-model";
import { LauncherPilePhysics } from "./launcher-agent-pile-physics";

export function useLauncherAgentPilePhysics(tokens: SpotlightToken[]) {
  const reducedMotion = usePrefersReducedMotion();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const tokenRefs = useRef<Record<string, HTMLElement | null>>({});
  const configs = useMemo(
    () => createTokenConfig(tokens, LAUNCHER_PILE_WIDTH),
    [tokens],
  );
  const configByKey = useMemo(
    () => new Map(configs.map((config) => [config.key, config])),
    [configs],
  );
  const tokenByKey = useMemo(
    () => new Map(tokens.map((token) => [token.key, token])),
    [tokens],
  );

  useEffect(() => {
    const container = containerRef.current;
    if (!container || tokenByKey.size === 0) {
      return;
    }
    const physics = new LauncherPilePhysics({
      configs,
      container,
      reducedMotion,
      tokenByKey,
      tokenRefs,
    });
    return () => physics.dispose();
  }, [configs, reducedMotion, tokenByKey]);

  const bindToken = useCallback((key: string, element: HTMLElement | null) => {
    tokenRefs.current[key] = element;
  }, []);

  return { bindToken, configByKey, containerRef };
}
