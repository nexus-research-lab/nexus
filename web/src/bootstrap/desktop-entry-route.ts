export function apply_desktop_entry_route(fallback_route: string) {
  if (typeof window === "undefined") {
    return;
  }

  const params = new URLSearchParams(window.location.search);
  const route = normalize_desktop_route(params.get("desktop_route"), fallback_route);
  window.history.replaceState(window.history.state, "", route);
}

function normalize_desktop_route(route: string | null, fallback_route: string): string {
  const candidate = (route ?? fallback_route).trim();
  if (!candidate.startsWith("/") || candidate.startsWith("//")) {
    return fallback_route;
  }
  return candidate;
}
