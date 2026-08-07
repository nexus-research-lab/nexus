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
  canSubmit,
  importing,
}: {
  canSubmit: boolean;
  importing: boolean;
}) {
  return (
    <UiButton
      disabled={!canSubmit}
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
  canSubmitGit,
  importing,
  mode,
  onClose,
}: {
  canSubmitGit: boolean;
  importing: boolean;
  mode: SkillImportDialogMode;
  onClose: () => void;
}) {
  const { t } = useI18n();
  return (
    <UiDialogFooter className="gap-2">
      <UiButton disabled={importing} onClick={onClose} size="sm" variant="surface">
        {t("common.cancel")}
      </UiButton>
      {mode === "git" ? (
        <GitImportSubmitButton
          canSubmit={canSubmitGit}
          importing={importing}
        />
      ) : null}
    </UiDialogFooter>
  );
}
