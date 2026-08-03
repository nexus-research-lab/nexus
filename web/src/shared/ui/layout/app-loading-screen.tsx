import { cn } from "@/shared/ui/class-name";

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
    <div
      aria-atomic="true"
      aria-live="polite"
      className={cn("flex flex-col items-center gap-3 px-12 py-10 text-center", className)}
      role="status"
    >
      <div
        aria-hidden="true"
        className={cn("relative grid place-items-center", animationClassName)}
      >
        <div className="absolute h-14 w-14 rounded-full border border-[color:color-mix(in_srgb,var(--primary)_16%,transparent)]" />
        <div className="absolute h-14 w-14 animate-spin rounded-full border-2 border-transparent border-t-primary motion-reduce:animate-none" />
        <div className="h-2 w-2 rounded-full bg-primary shadow-[0_0_16px_color-mix(in_srgb,var(--primary)_48%,transparent)]" />
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
