export function applyDesktopEntryRoute(fallbackRoute: string) {
  if (typeof window === "undefined") {
    return;
  }

  const params = new URLSearchParams(window.location.search);
  const route = normalizeDesktopRoute(params.get("desktop_route"), fallbackRoute);
  window.history.replaceState(window.history.state, "", route);
}

function normalizeDesktopRoute(route: string | null, fallbackRoute: string): string {
  const candidate = (route ?? fallbackRoute).trim();
  if (!candidate.startsWith("/") || candidate.startsWith("//")) {
    return fallbackRoute;
  }
  try {
    const resolved = new URL(candidate, window.location.origin);
    if (resolved.origin !== window.location.origin) return fallbackRoute;
    return `${resolved.pathname}${resolved.search}${resolved.hash}`;
  } catch {
    return fallbackRoute;
  }
}
