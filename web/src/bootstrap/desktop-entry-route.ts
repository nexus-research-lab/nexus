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
  return candidate;
}
