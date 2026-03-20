"use client";

export function HomeLoadingScreen() {
  return (
    <main className="relative flex h-screen w-full items-center justify-center overflow-hidden bg-background px-6 text-foreground">
      <div className="pointer-events-none absolute left-[10%] top-[12%] h-56 w-56 rounded-full glow-lilac opacity-70" />
      <div className="pointer-events-none absolute bottom-[10%] right-[12%] h-64 w-64 rounded-full glow-green opacity-60" />
      <div className="soft-ring radius-shell-lg panel-surface px-10 py-9 text-center">
        <div className="mx-auto h-10 w-10 animate-spin rounded-full border-4 border-primary/20 border-t-primary" />
        <p className="mt-4 text-sm text-muted-foreground">正在加载...</p>
      </div>
    </main>
  );
}
