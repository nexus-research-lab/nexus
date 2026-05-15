import { AppProviders } from "@/app/app-providers";
import { DesktopLauncherRouter } from "@/app/router/desktop-launcher-router";
import { apply_desktop_entry_route } from "@/bootstrap/desktop-entry-route";
import { bootstrap_react_app } from "@/bootstrap/root-bootstrap";

apply_desktop_entry_route("/?desktop_surface=launcher");
bootstrap_react_app(() => (
  <AppProviders>
    <DesktopLauncherRouter />
  </AppProviders>
));
