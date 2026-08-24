import { Download, Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { UiDialogFooter } from "@/shared/ui/dialog/dialog";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";

function GitImportStatus({ importing }: { importing: boolean }) {
  const { t } = useI18n();
  return importing ? (
    <>
      <Loader2 className="h-4 w-4 animate-spin" />
      {t("capability.skills_importing")}
    </>
  ) : (
    <>
      <Download className="h-4 w-4" />
      {t("capability.skills_import_git_submit")}
    </>
  );
}

function GitImportSubmitButton({
  importing,
}: {
  importing: boolean;
}) {
  return (
    <UiButton
      disabled={importing}
      size="sm"
      tone="primary"
      type="submit"
      variant="solid"
    >
      <GitImportStatus importing={importing} />
    </UiButton>
  );
}

export function SkillImportFooter({
  importing,
  mode,
  onClose,
}: {
  importing: boolean;
  mode: SkillImportDialogMode;
  onClose: () => void;
}) {
  const { t } = useI18n();
  return (
    <UiDialogFooter appearance="plain" className="gap-2">
      <UiButton disabled={importing} onClick={onClose} size="sm" variant="surface">
        {t("common.cancel")}
      </UiButton>
      {mode === "git" ? (
        <GitImportSubmitButton
          importing={importing}
        />
      ) : null}
    </UiDialogFooter>
  );
}
/**
 * INPUT: 当前导入模式、在途状态与关闭动作。
 * OUTPUT: 只保留取消和当前模式主动作的 plain Footer。
 * POS: Skill 导入弹窗动作区。
 */
