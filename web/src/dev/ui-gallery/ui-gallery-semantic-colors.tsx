// INPUT: 当前主题、语言与公共控件的有限语义 tone / variant。
// OUTPUT: 页面、卡片和浮层上真实文字、Badge、Button 与选择项的对比度审查面。
// POS: 开发期颜色夹具；只组合公共实现，不复制颜色或模拟业务状态。

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, type UiButtonTone, type UiButtonVariant } from "@/shared/ui/button/button";
import { UiBadge, UiCounterBadge } from "@/shared/ui/display/badge";
import type { UiBadgeTone } from "@/shared/ui/display/badge-styles";
import { UiChoiceButton } from "@/shared/ui/form/choice";
import { UiField, UiInput } from "@/shared/ui/form/form-control";
import { getUiTypographyClassName, type UiTypographyTone } from "@/shared/ui/typography/typography-styles";

import { galleryText } from "./ui-gallery-copy";

const surfaces = { page: "bg-background", card: "bg-(--card)", overlay: "surface-popover" };
const badgeTones: UiBadgeTone[] = ["default", "primary", "success", "warning", "danger", "info", "idle", "active", "running"];
const textTones: UiTypographyTone[] = ["strong", "default", "muted", "soft", "brand", "danger", "success", "warning"];
const buttonTones: UiButtonTone[] = ["default", "primary", "danger", "success"];
const buttonVariants: UiButtonVariant[] = ["surface", "outline", "solid", "ghost", "text"];

export function SemanticColorsGallery() {
  const { locale } = useI18n();
  return <section className="min-w-0 space-y-4 bg-background xl:col-span-2" data-gallery-semantic-colors>
    <h2 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
      {galleryText(locale, "语义颜色与实际表面", "Semantic colors on real surfaces")}
    </h2>
    {Object.entries(surfaces).map(([surface, className]) => <section key={surface}
      className={`min-w-0 space-y-4 p-3 ${className}`} data-gallery-color-surface={surface}>
      <h3 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>{surface}</h3>
      <div className="flex flex-wrap items-center gap-2">
        {badgeTones.map((tone) => <UiBadge data-color-sample={`badge:${tone}`} key={tone} tone={tone}>{tone}</UiBadge>)}
        <UiCounterBadge count={120} data-color-sample="counter" />
      </div>
      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        {textTones.map((tone) => <span className={getUiTypographyClassName({ role: "supporting", tone })}
          data-color-sample={`text:${tone}`} key={tone}>{tone} · {galleryText(locale, "状态说明", "Status details")}</span>)}
      </div>
      <div className="grid grid-cols-2 gap-2 md:grid-cols-5" data-gallery-color-buttons>
        {buttonTones.flatMap((tone) => buttonVariants.map((variant) => <UiButton data-color-sample={`button:${tone}:${variant}`}
          key={`${tone}:${variant}`} size="sm" tone={tone} variant={variant}>{tone} · {variant}</UiButton>))}
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <UiField label={galleryText(locale, "字段校验", "Field validation")} htmlFor={`color-field-${surface}`}
          error={<span data-color-sample="field-error">{galleryText(locale, "请填写有效的名称。", "Enter a valid name.")}</span>}>
          <UiInput id={`color-field-${surface}`} defaultValue="Nexus" />
        </UiField>
        <div className="flex flex-wrap items-center gap-2">
          <UiChoiceButton active data-color-sample="choice:primary" tone="primary">{galleryText(locale, "已选择", "Selected")}</UiChoiceButton>
          <UiChoiceButton active data-color-sample="choice:neutral" tone="neutral">{galleryText(locale, "当前", "Current")}</UiChoiceButton>
          <UiChoiceButton active data-color-sample="choice:danger" tone="danger">{galleryText(locale, "需要处理", "Needs attention")}</UiChoiceButton>
          <UiChoiceButton active data-color-sample="choice:success" tone="success">{galleryText(locale, "已确认", "Confirmed")}</UiChoiceButton>
        </div>
      </div>
    </section>)}
  </section>;
}
