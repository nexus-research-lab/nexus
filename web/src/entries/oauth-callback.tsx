import { DesktopOAuthCallbackRouter } from "@/app/router/desktop-oauth-callback-router";
import { applyDesktopEntryRoute } from "@/bootstrap/desktop-entry-route";
import { bootstrapPublicReactApp } from "@/bootstrap/root-bootstrap";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ThemeProvider } from "@/shared/theme/theme-provider";

applyDesktopEntryRoute("/capability/connectors/oauth/callback");
bootstrapPublicReactApp(() => (
  <ThemeProvider>
    <I18nProvider>
      <DesktopOAuthCallbackRouter />
    </I18nProvider>
  </ThemeProvider>
));
