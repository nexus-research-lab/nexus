import { AppRouter } from "@/app/router/app-router";
import { AppProviders } from "@/app/app-providers";

export function App() {
  return (
    <AppProviders>
      <AppRouter />
    </AppProviders>
  );
}
