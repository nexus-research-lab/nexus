import { AppRouter } from "@/app/router/app-router";
import { AuthProvider } from "@/shared/auth/auth-provider";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ThemeProvider } from "@/shared/theme/theme-provider";

export function App() {
  return (
    <ThemeProvider>
      <I18nProvider>
        <AuthProvider>
          <AppRouter />
        </AuthProvider>
      </I18nProvider>
    </ThemeProvider>
  );
}
