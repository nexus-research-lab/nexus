import {
  type FormEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import type { SkillImportDialogMode } from "../controller/skill-marketplace-controller";
import {
  canSubmitGitSkillImport,
  EMPTY_GIT_SKILL_IMPORT_DRAFT,
  type GitSkillImportDraft,
} from "./skill-import-dialog-model";

interface UseSkillImportDialogOptions {
  importing: boolean;
  mode: SkillImportDialogMode | null;
  onClose: () => void;
  onImportGit: (url: string, branch?: string, path?: string) => void;
}

export function useSkillImportDialog({
  importing,
  mode,
  onClose,
  onImportGit,
}: UseSkillImportDialogOptions) {
  const [draft, setDraft] = useState<GitSkillImportDraft>(
    EMPTY_GIT_SKILL_IMPORT_DRAFT,
  );
  const gitUrlInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    // 关闭时清理草稿；来源切换时保留输入，避免误触标签造成内容丢失。
    if (!mode) {
      setDraft(EMPTY_GIT_SKILL_IMPORT_DRAFT);
    }
  }, [mode]);

  useEffect(() => {
    if (mode === "git") {
      gitUrlInputRef.current?.focus();
    }
  }, [mode]);

  const close = useCallback(() => {
    if (!importing) {
      onClose();
    }
  }, [importing, onClose]);

  const setDraftField = useCallback(<Key extends keyof GitSkillImportDraft>(
    key: Key,
    value: GitSkillImportDraft[Key],
  ) => {
    setDraft((current) => ({ ...current, [key]: value }));
  }, []);

  const submit = useCallback((event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitGitSkillImport(mode, importing, draft)) {
      return;
    }
    onImportGit(draft.url, draft.branch, draft.path);
  }, [draft, importing, mode, onImportGit]);

  return {
    canSubmitGit: canSubmitGitSkillImport(mode, importing, draft),
    close,
    dismissAction: importing ? undefined : close,
    draft,
    gitUrlInputRef,
    setDraftField,
    submit,
  };
}
