"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import type { SpotlightToken } from "@/types/app/launcher";

import { createTokenConfig } from "./launcher-agent-pile-model";
import { LauncherPilePhysics } from "./launcher-agent-pile-physics";

export function useLauncherAgentPilePhysics(tokens: SpotlightToken[]) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const tokenRefs = useRef<Record<string, HTMLElement | null>>({});
  const configs = useMemo(() => createTokenConfig(tokens, 560), [tokens]);
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
      tokenByKey,
      tokenRefs,
    });
    return () => physics.dispose();
  }, [configs, tokenByKey]);

  const bindToken = useCallback((key: string, element: HTMLElement | null) => {
    tokenRefs.current[key] = element;
  }, []);

  return { bindToken, configByKey, containerRef };
}
