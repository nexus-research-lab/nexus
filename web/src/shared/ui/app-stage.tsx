import { ReactNode } from "react";

import { HOME_PAGE_PADDING_CLASS } from "@/lib/home-layout";

interface AppStageProps {
  children: ReactNode;
}

export function AppStage({ children }: AppStageProps) {
  return (
    <main className="relative flex h-screen w-full overflow-hidden bg-background text-foreground bg-[radial-gradient(circle_at_top,rgba(255,255,255,0.36),transparent_38%),radial-gradient(circle_at_bottom,rgba(255,255,255,0.24),transparent_32%)]">
      <div className="pointer-events-none absolute left-[5%] top-[8%] h-72 w-72 rounded-full glow-lilac opacity-55" />
      <div className="pointer-events-none absolute bottom-[6%] left-[22%] h-80 w-80 rounded-full bg-white/40 blur-3xl" />
      <div className="pointer-events-none absolute right-[8%] top-[18%] h-72 w-72 rounded-full glow-peach opacity-35" />
      <div className="pointer-events-none absolute right-[12%] bottom-[8%] h-80 w-80 rounded-full glow-green opacity-40" />

      <div className={`relative flex min-h-0 flex-1 flex-col ${HOME_PAGE_PADDING_CLASS}`}>{children}</div>
    </main>
  );
}
