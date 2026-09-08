// INPUT: Optional loading title and layout placement supplied by a file preview.
// OUTPUT: One named preview loading surface using shared ResourceState and canvas-sized Spinner.
// POS: File-preview state composition; no fetching, parsing, retry or document identity.

import { LoaderCircle } from "lucide-react";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function WorkspaceFilePreviewLoading({ className, title }: { className?: string; title?: string }) {
  const { t } = useI18n();
  return <UiResourceState
    className={cn("min-h-0 w-full", className)}
    icon={<LoaderCircle aria-hidden className={getUiSpinnerClassName({ size: "2xl", tone: "muted" })} />}
    size="sm" state="loading" title={title ?? t("workspace_file.preview_loading")} variant="plain"
  />;
}
