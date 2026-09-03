// INPUT: Gallery 当前语言与中英文 fixture 文案。
// OUTPUT: 只选择文案、不复制组件结构或视觉样式的本地化结果。
// POS: 开发期 Gallery 文案辅助；不替代产品 i18n，也不参与产品构建。

import type { Locale } from "@/shared/i18n/messages";

export function galleryText(locale: Locale, zh: string, en: string): string {
  return locale === "zh" ? zh : en;
}
