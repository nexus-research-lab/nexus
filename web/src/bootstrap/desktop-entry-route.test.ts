// INPUT: Desktop entry URL supplied to a same-origin browser history replacement.
// OUTPUT: Host-local navigation or safe fallback before React startup.
// POS: Desktop entry route normalization regression.
import { afterEach, expect, it } from "vitest";
import { applyDesktopEntryRoute } from "./desktop-entry-route";
afterEach(() => { window.history.replaceState(null, "", "/"); });
it("keeps a host-local route query and fragment", () => {
  history.replaceState(null, "", "/?desktop_route=" + encodeURIComponent("/settings?section=appearance#fonts"));
  applyDesktopEntryRoute("/app");
  expect(location.pathname + location.search + location.hash).toBe("/settings?section=appearance#fonts");
});
it("falls back instead of throwing before startup on a backslash origin escape", () => {
  history.replaceState(null, "", "/?desktop_route=" + encodeURIComponent("/\\example.com/settings"));
  expect(() => applyDesktopEntryRoute("/app")).not.toThrow();
  expect(location.pathname).toBe("/app");
});
