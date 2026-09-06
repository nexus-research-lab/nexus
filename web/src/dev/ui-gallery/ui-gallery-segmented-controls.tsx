// INPUT: 当前语言、分段控件的两档密度和三种内容模式。
// OUTPUT: 使用真实组件演示选中/未选中、键盘焦点、禁用与图标文字排列。
// POS: 开发期选择控件夹具；本地值与命令记录不触发产品配置写入。

import { Code2, Eye } from "lucide-react";
import { useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiSegmentedControl } from "@/shared/ui/form/segmented-control";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { galleryText } from "./ui-gallery-copy";

export function SegmentedControlsGallery() {
  const { locale } = useI18n();
  const [disabled, setDisabled] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({});
  const [commands, setCommands] = useState<string[]>([]);
  return <section className="min-w-0 space-y-3 bg-background" data-gallery-segmented>
    <h2 className={getUiTypographyClassName({ role: "sectionTitle", tone: "strong" })}>
      {galleryText(locale, "分段选择与交互状态", "Segmented choices and interaction states")}
    </h2>
    <UiButton data-gallery-segmented-lock onClick={() => setDisabled((current) => !current)} size="sm" variant="surface">
      {disabled ? galleryText(locale, "启用控件", "Enable controls") : galleryText(locale, "禁用控件", "Disable controls")}
    </UiButton>
    {(["default", "compact"] as const).flatMap((density) => ["text", "mixed", "icons"].map((mode) => {
      const key = `${density}-${mode}`;
      return <div className="space-y-1" data-segmented-case={key} key={key}>
        <p className={getUiTypographyClassName({ role: "metadata", tone: "muted" })}>{key}</p>
        <UiSegmentedControl density={density} disabled={disabled}
          onChange={(value) => {
            setValues((current) => ({ ...current, [key]: value }));
            setCommands((current) => [...current, `${key}:${value}`]);
          }}
          options={[
            { icon: mode !== "text" ? Eye : undefined, iconOnly: mode === "icons", label: galleryText(locale, "预览", "Preview"), value: "preview" },
            { icon: mode !== "text" ? Code2 : undefined, iconOnly: mode === "icons", label: galleryText(locale, "源码", "Source"), value: "source" },
          ]}
          title={`${galleryText(locale, "显示方式", "Display mode")} · ${key}`} value={values[key] ?? "preview"} />
      </div>;
    }))}
    <output className="sr-only" data-gallery-segmented-commands>{JSON.stringify(commands)}</output>
  </section>;
}
