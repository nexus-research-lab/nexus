import { App } from "@/App";
import { AppProviders } from "@/app/app-providers";
import { DesktopSettingsRouter } from "@/app/router/desktop-settings-router";
import { apply_desktop_entry_route } from "@/bootstrap/desktop-entry-route";
import { bootstrap_react_app } from "@/bootstrap/root-bootstrap";
import { is_desktop_runtime } from "@/config/desktop-runtime";

if (is_desktop_runtime()) {
  apply_desktop_entry_route("/settings");
  bootstrap_react_app(() => (
    <AppProviders>
      <DesktopSettingsRouter />
    </AppProviders>
  ));
} else {
  bootstrap_react_app(() => <App />);
}
