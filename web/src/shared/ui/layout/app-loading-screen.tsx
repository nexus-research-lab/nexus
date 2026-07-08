"use client";

import { ANIMATIONS } from "@/config/animation-assets";
import { cn } from "@/shared/ui/class-name";
import { LottiePlayer } from "@/shared/ui/feedback/lottie-player";

interface AppLoadingStateProps {
  className?: string;
  animationClassName?: string;
  message?: string;
}

export function AppLoadingState({
  className: className,
  animationClassName: animationClassName = "h-32 w-32 shrink-0",
  message = "正在加载...",
}: AppLoadingStateProps) {
  return (
    <div className={cn("flex flex-col items-center gap-3 px-12 py-10 text-center", className)}>
      <LottiePlayer
        className={animationClassName}
        src={ANIMATIONS.CAT}
      />
      <p className="text-sm text-(--text-muted)">{message}</p>
    </div>
  );
}

export function AppLoadingScreen() {
  return (
    <main className="relative flex h-screen w-full items-center justify-center overflow-hidden bg-background px-6 text-foreground">
      <AppLoadingState />
    </main>
  );
}
