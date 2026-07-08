import { AppProviders } from "@/app/app-providers";
import { DesktopOAuthCallbackRouter } from "@/app/router/desktop-oauth-callback-router";
import { applyDesktopEntryRoute } from "@/bootstrap/desktop-entry-route";
import { bootstrapReactApp } from "@/bootstrap/root-bootstrap";

applyDesktopEntryRoute("/capability/connectors/oauth/callback");
bootstrapReactApp(() => (
  <AppProviders>
    <DesktopOAuthCallbackRouter />
  </AppProviders>
));
