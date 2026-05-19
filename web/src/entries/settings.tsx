import { AppProviders } from "@/app/app-providers";
import { DesktopSettingsRouter } from "@/app/router/desktop-settings-router";
import { apply_desktop_entry_route } from "@/bootstrap/desktop-entry-route";
import { bootstrap_react_app } from "@/bootstrap/root-bootstrap";

apply_desktop_entry_route("/settings");
bootstrap_react_app(() => (
  <AppProviders>
    <DesktopSettingsRouter />
  </AppProviders>
));
