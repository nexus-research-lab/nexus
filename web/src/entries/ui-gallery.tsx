// INPUT: 开发者打开 ui-gallery.html 时提供的 theme/locale 查询参数。
// OUTPUT: 不依赖登录态与业务 API 的 shared UI 契约陈列面。
// POS: 仅供 Vite 开发服务器使用的独立入口；不得加入生产 Rollup inputs。

import { UiContractGallery } from "@/dev/ui-gallery/ui-contract-gallery";
import { bootstrapPublicReactApp } from "@/bootstrap/root-bootstrap";
import { I18nProvider } from "@/shared/i18n/i18n-provider";
import { LOCALE_STORAGE_KEY } from "@/shared/i18n/messages";
import {
  THEME_STORAGE_KEY,
  type VisualTheme,
} from "@/shared/theme/theme-context";
import { ThemeProvider } from "@/shared/theme/theme-provider";

const VISUAL_THEMES = new Set<VisualTheme>(["light", "dark", "rain"]);

function applyGalleryQueryPreferences(): void {
  const query = new URLSearchParams(window.location.search);
  const theme = query.get("theme") as VisualTheme | null;
  const locale = query.get("locale");

  if (theme && VISUAL_THEMES.has(theme)) {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  }
  if (locale === "zh" || locale === "en") {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  }
}

applyGalleryQueryPreferences();

bootstrapPublicReactApp(() => (
  <ThemeProvider>
    <I18nProvider>
      <UiContractGallery />
    </I18nProvider>
  </ThemeProvider>
));
