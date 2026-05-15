import { AppProviders } from "@/app/app-providers";
import { DesktopOAuthCallbackRouter } from "@/app/router/desktop-oauth-callback-router";
import { apply_desktop_entry_route } from "@/bootstrap/desktop-entry-route";
import { bootstrap_react_app } from "@/bootstrap/root-bootstrap";

apply_desktop_entry_route("/capability/connectors/oauth/callback");
bootstrap_react_app(() => (
  <AppProviders>
    <DesktopOAuthCallbackRouter />
  </AppProviders>
));
