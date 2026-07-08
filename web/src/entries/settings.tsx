import { App } from "@/App";
import { AppProviders } from "@/app/app-providers";
import { DesktopSettingsRouter } from "@/app/router/desktop-settings-router";
import { applyDesktopEntryRoute } from "@/bootstrap/desktop-entry-route";
import { bootstrapReactApp } from "@/bootstrap/root-bootstrap";
import { isDesktopRuntime } from "@/config/desktop-runtime";

if (isDesktopRuntime()) {
  applyDesktopEntryRoute("/settings");
  bootstrapReactApp(() => (
    <AppProviders>
      <DesktopSettingsRouter />
    </AppProviders>
  ));
} else {
  bootstrapReactApp(() => <App />);
}
