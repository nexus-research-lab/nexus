/**
 * INPUT: 当前语言与导入在途状态。
 * OUTPUT: 默认收起的 Skill 格式要求、示例与指南下载动作。
 * POS: Skill 导入表单的次级帮助，不占据首屏主栏。
 */

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiDisclosure } from "@/shared/ui/disclosure/disclosure";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { Locale } from "@/shared/i18n/messages";
import { UiPanel } from "@/shared/ui/panel";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import roomCollaborationMechanismEnglishMarkdown from "../../../../../../docs/guides/room-skill-authoring.en.md?raw";
import roomCollaborationMechanismChineseMarkdown from "../../../../../../docs/guides/room-skill-authoring.md?raw";
import { buildSkillFrontmatterExample } from "./skill-import-dialog-model";

const ROOM_COLLABORATION_GUIDES: Record<Locale, {
  content: string;
  fileName: string;
}> = {
  en: {
    content: roomCollaborationMechanismEnglishMarkdown,
    fileName: "room-skill-authoring.en.md",
  },
  zh: {
    content: roomCollaborationMechanismChineseMarkdown,
    fileName: "room-skill-authoring.md",
  },
};

function downloadRoomCollaborationMechanism(locale: Locale) {
  const guide = ROOM_COLLABORATION_GUIDES[locale];
  const blob = new Blob([guide.content], {
    type: "text/markdown;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = guide.fileName;
  anchor.rel = "noopener";
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export function SkillImportGuide({ importing }: { importing: boolean }) {
  const { locale, t } = useI18n();
  return (
    <aside className="border-t border-(--divider-subtle-color) pt-4">
      <UiDisclosure
        contentClassName="space-y-3 pl-4"
        label={t("capability.skills_import_guide_title")}
        summaryTone="muted"
        variant="inline"
      >
          <div className="flex justify-end">
            <UiButton
              aria-label={t("capability.skills_import_guide_download_aria")}
              className="shrink-0"
              disabled={importing}
              onClick={() => downloadRoomCollaborationMechanism(locale)}
              size="xs"
              tone="primary"
              variant="text"
            >
              {t("capability.skills_import_guide_download")}
            </UiButton>
          </div>
          <ul className={cn(
            "space-y-1.5",
            getUiTypographyClassName({ role: "caption", tone: "muted" }),
          )}>
            <li>{t("capability.skills_import_rule_name")}</li>
            <li>{t("capability.skills_import_rule_scope")}</li>
            <li>{t("capability.skills_import_rule_room_guide")}</li>
            <li>{t("capability.skills_import_rule_room_enable")}</li>
            <li>{t("capability.skills_import_rule_git_tracking")}</li>
          </ul>
          <UiPanel
            className="bg-[color:color-mix(in_srgb,var(--background)_92%,black_2%)]"
            padding="sm"
            radius="sm"
            variant="card"
          >
            <pre className={cn(
              "max-h-[260px] overflow-auto whitespace-pre-wrap",
              getUiTypographyClassName({ role: "code", tone: "default" }),
            )}>
              {buildSkillFrontmatterExample(t)}
            </pre>
          </UiPanel>
      </UiDisclosure>
    </aside>
  );
}
