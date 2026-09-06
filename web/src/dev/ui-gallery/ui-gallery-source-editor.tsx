// INPUT: Local source draft, locale and native read-only/disabled states.
// OUTPUT: Real source primitives, Field actions, scrolling and keyboard focus fixtures.
// POS: Development-only source surface; recording a draft only updates local fixture output.

import { useState } from "react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { UiField } from "@/shared/ui/form/form-control";
import { UiSourceEditor } from "@/shared/ui/form/source-editor";
import { UiPanel } from "@/shared/ui/panel";
import { galleryText } from "./ui-gallery-copy";

export function SourceEditorGallery() {
  const { locale } = useI18n();
  const [draft, setDraft] = useState("# Source · 源码\n\nconst value = 'keep  spaces';\n\t中文输入");
  const [recorded, setRecorded] = useState("");
  return <section className="grid min-w-0 gap-4" data-gallery-source-editor>
    <UiField htmlFor="gallery-source-draft" label={galleryText(locale, "源码草稿", "Source draft")}
      description={galleryText(locale, "保留空格、换行和原生输入法。", "Preserves spaces, newlines and native composition.")}
      labelAction={<UiButton onClick={() => setRecorded(draft)} size="sm">{galleryText(locale, "记录草稿", "Record draft")}</UiButton>}>
      <UiPanel className="flex h-44 min-w-0 p-3">
        <UiSourceEditor id="gallery-source-draft" onChange={(event) => setDraft(event.target.value)} value={draft} />
      </UiPanel>
    </UiField>
    <UiField htmlFor="gallery-source-saved" label={galleryText(locale, "只读源码", "Read-only source")}>
      <UiPanel className="flex h-28 min-w-0 p-3">
        <UiSourceEditor defaultValue="Saved  source\n\t只读内容" id="gallery-source-saved" readOnly />
      </UiPanel>
    </UiField>
    <UiField htmlFor="gallery-source-disabled" label={galleryText(locale, "不可编辑的源码", "Disabled source")}>
      <UiPanel className="flex h-28 min-w-0 p-3">
        <UiSourceEditor defaultValue="Unavailable source" disabled id="gallery-source-disabled" />
      </UiPanel>
    </UiField>
    <output className="sr-only" data-gallery-source-record>{JSON.stringify(recorded)}</output>
  </section>;
}
