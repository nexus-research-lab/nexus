// INPUT: Provider-free locale preference.
// OUTPUT: Minimal safe root recovery copy.
// POS: Bootstrap fallback text; independent of the application catalog.
import { detectInitialLocale } from "@/shared/i18n/locale-settings";

export function getRootFailureCopy() {
  return detectInitialLocale() === "en"
    ? { retry: "Retry", message: "Please try again shortly.", startup: "Unable to start", page: "Unable to display this page" }
    : { retry: "重试", message: "请稍后重试。", startup: "暂时无法启动", page: "页面暂时无法显示" };
}
