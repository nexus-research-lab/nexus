// INPUT: 当前导入模式、在途状态与关闭动作。
// OUTPUT: 直接组合取消、Git 提交与显式 busy 状态的 plain Footer。
// POS: Skill 导入弹窗动作区；按钮和加载状态服从 shared/ui。

import { Download, Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiDialogFooter } from "@/shared/ui/dialog/dialog";
import { useI18n } from "@/shared/i18n/i18n-context";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";

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
        <UiButton aria-busy={importing || undefined} disabled={importing} size="sm" tone="primary" type="submit" variant="solid">
          {importing ? <Loader2 className={getUiSpinnerClassName()} /> : <Download className="h-4 w-4" />}
          {t(importing ? "capability.skills_importing" : "capability.skills_import_git_submit")}
        </UiButton>
      ) : null}
    </UiDialogFooter>
  );
}
