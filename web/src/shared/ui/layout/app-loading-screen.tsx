"use client";

import { cn } from "@/lib/utils";

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
        <div
          aria-hidden="true"
          className={cn(
            "relative rounded-full border border-(--surface-panel-border) bg-(--surface-panel-subtle-background)",
            "after:absolute after:inset-3 after:rounded-full after:border-2 after:border-primary after:border-t-transparent after:content-[''] after:animate-spin",
            animation_class_name,
          )}
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
