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
      <picture className={cn("block", animationClassName)}>
        <source
          media="(prefers-reduced-motion: reduce)"
          srcSet="/lotties/cat-loading-static.webp"
          type="image/webp"
        />
        <img
          alt=""
          className="h-full w-full object-contain"
          decoding="async"
          src="/lotties/cat-loading.webp"
        />
      </picture>
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
