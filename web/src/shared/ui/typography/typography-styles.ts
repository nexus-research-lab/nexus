// INPUT: App chrome 的稳定文本角色、可选语义 tone 与有限字重覆盖。
// OUTPUT: 指向 theme recipe 的排版 class；不改变调用方选择的 HTML 语义标签。
// POS: shared/ui 排版所有权；阅读正文、品牌字形和图形内文字仍由所属 Surface 负责。

export type UiTypographyRole =
  | "display"
  | "featureTitle"
  | "objectTitle"
  | "pageTitle"
  | "sectionTitle"
  | "body"
  | "control"
  | "supporting"
  | "metadata"
  | "caption"
  | "overline"
  | "code";

export type UiTypographyTone =
  | "inherit"
  | "strong"
  | "default"
  | "muted"
  | "soft"
  | "brand"
  | "danger"
  | "success"
  | "warning";

export type UiTypographyWeight = "regular" | "medium" | "semibold";

export interface UiTypographyOptions {
  role: UiTypographyRole;
  tone?: UiTypographyTone;
  weight?: UiTypographyWeight;
}

const ROLE_CLASS_NAMES: Record<UiTypographyRole, string> = {
  display: "ui-type-display",
  featureTitle: "ui-type-feature-title",
  objectTitle: "ui-type-object-title",
  pageTitle: "ui-type-page-title",
  sectionTitle: "ui-type-section-title",
  body: "ui-type-body",
  control: "ui-type-control",
  supporting: "ui-type-supporting",
  metadata: "ui-type-metadata",
  caption: "ui-type-caption",
  overline: "ui-type-overline",
  code: "ui-type-code",
};

const TONE_CLASS_NAMES: Record<Exclude<UiTypographyTone, "inherit">, string> = {
  strong: "ui-type-tone-strong",
  default: "ui-type-tone-default",
  muted: "ui-type-tone-muted",
  soft: "ui-type-tone-soft",
  brand: "ui-type-tone-brand",
  danger: "ui-type-tone-danger",
  success: "ui-type-tone-success",
  warning: "ui-type-tone-warning",
};

const WEIGHT_CLASS_NAMES: Record<UiTypographyWeight, string> = {
  regular: "ui-type-weight-regular",
  medium: "ui-type-weight-medium",
  semibold: "ui-type-weight-semibold",
};

export function getUiTypographyClassName({
  role,
  tone = "inherit",
  weight,
}: UiTypographyOptions): string {
  return [
    ROLE_CLASS_NAMES[role],
    tone === "inherit" ? "" : TONE_CLASS_NAMES[tone],
    weight ? WEIGHT_CLASS_NAMES[weight] : "",
  ].filter(Boolean).join(" ");
}
