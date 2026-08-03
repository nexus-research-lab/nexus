import { isDesktopRuntime } from "@/config/desktop-runtime";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";

export interface WorkspaceFileExternalActionCopy {
  ariaLabel: string;
  label: string;
  mode: "download" | "reveal";
  title: string;
}

export const getWorkspaceFileExternalActionCopy = (
  translate: I18nContextValue["t"],
  fileName?: string,
): WorkspaceFileExternalActionCopy => {
  const normalizedFileName = fileName?.trim()
    || translate("workspace_file.default_name");
  if (isDesktopRuntime()) {
    const title = translate("workspace_file.reveal_named", {
      name: normalizedFileName,
    });
    return {
      ariaLabel: title,
      label: translate("workspace_file.open"),
      mode: "reveal",
      title,
    };
  }
  const title = translate("workspace_file.download_named", {
    name: normalizedFileName,
  });
  return {
    ariaLabel: title,
    label: translate("workspace_file.download"),
    mode: "download",
    title,
  };
};
