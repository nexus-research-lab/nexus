"use client";

import { ANIMATIONS } from "@/config/animation-assets";
import { cn } from "@/lib/utils";
import { LottiePlayer } from "../feedback/lottie-player";

interface AppLoadingStateProps {
  class_name?: string;
  animation_class_name?: string;
  message?: string;
}

export function AppLoadingState({
  class_name,
  animation_class_name = "h-32 w-32 shrink-0",
  message = "正在加载...",
}: AppLoadingStateProps) {
  return (
    <div className={cn("surface-panel radius-shell-lg flex flex-col items-center gap-3 px-12 py-10 text-center", class_name)}>
      <LottiePlayer
        class_name={animation_class_name}
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
