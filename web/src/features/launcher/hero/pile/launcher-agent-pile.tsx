"use client";

import { cn } from "@/lib/utils";
import type { SpotlightToken } from "@/types/app/launcher";

import { LauncherAgentToken } from "./launcher-agent-token";
import { useLauncherAgentPilePhysics } from "./use-launcher-agent-pile-physics";

interface SpotlightTokenPileProps {
  className?: string;
  currentAgentId: string | null;
  onSelectAgent: (agentId: string) => void;
  tokens: SpotlightToken[];
}

export function AgentPile({
  className,
  currentAgentId,
  onSelectAgent,
  tokens,
}: SpotlightTokenPileProps) {
  const physics = useLauncherAgentPilePhysics(tokens);
  return (
    <div
      className={cn(
        "pointer-events-none relative z-0 mt-14 h-[286px] w-full max-w-[640px] overflow-hidden mask-[linear-gradient(180deg,transparent_0,black_14%,black_92%,transparent_100%)]",
        className,
      )}
      ref={physics.containerRef}
    >
      <div className="pointer-events-none absolute bottom-[34px] left-1/2 h-[114px] w-[128%] -translate-x-1/2 rounded-[999px] border-t border-white/22 bg-[radial-gradient(circle_at_50%_8%,rgba(255,255,255,0.14),rgba(255,255,255,0.03)_28%,rgba(255,255,255,0)_62%)]" />
      <div className="pointer-events-none absolute inset-x-0 top-[194px] h-px bg-[linear-gradient(90deg,rgba(255,255,255,0),rgba(255,255,255,0.1),rgba(255,255,255,0.3),rgba(255,255,255,0.1),rgba(255,255,255,0))]" />

      {tokens.map((token) => {
        const config = physics.configByKey.get(token.key);
        if (!config) {
          return null;
        }
        return (
          <LauncherAgentToken
            bindElement={(element) => physics.bindToken(token.key, element)}
            config={config}
            isActive={Boolean(
              token.agent_id && token.agent_id === currentAgentId,
            )}
            key={token.key}
            onSelectAgent={onSelectAgent}
            token={token}
          />
        );
      })}
    </div>
  );
}
