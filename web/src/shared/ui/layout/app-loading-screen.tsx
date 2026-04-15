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
    <div className={cn("flex flex-col items-center gap-3 px-12 py-10 text-center", class_name)}>
      <div className="relative isolate flex items-center justify-center">
        <div className="pointer-events-none absolute inset-4 rounded-full bg-[radial-gradient(circle,rgba(255,170,118,0.2)_0%,rgba(123,166,255,0.12)_45%,rgba(255,255,255,0)_78%)] blur-2xl" />
        <LottiePlayer
          class_name={cn("relative drop-shadow-[0_18px_36px_rgba(34,48,89,0.16)]", animation_class_name)}
          src={ANIMATIONS.CAT}
        />
      </div>
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
