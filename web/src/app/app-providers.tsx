import { ReactNode } from "react";

import { AuthProvider } from "@/shared/auth/auth-provider";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { ThemeProvider } from "@/shared/theme/theme-provider";

export function AppProviders({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <I18nProvider>
        <AuthProvider>{children}</AuthProvider>
      </I18nProvider>
    </ThemeProvider>
  );
}
