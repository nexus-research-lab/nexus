import { Download, Loader2 } from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { UiDialogFooter } from "@/shared/ui/dialog/dialog";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";

function GitImportStatus({ importing }: { importing: boolean }) {
  return importing ? (
    <>
      <Loader2 className="h-4 w-4 animate-spin" />
      导入中
    </>
  ) : (
    <>
      <Download className="h-4 w-4" />
      导入 Git Skill
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
  return (
    <UiDialogFooter className="gap-2">
      <UiButton disabled={importing} onClick={onClose} size="sm" variant="surface">
        取消
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
