import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { ExternalSkillSearchItem } from "@/types/capability/skill";

type ExternalSkillImportKind = "available" | "conflict" | "imported";

export interface ExternalSkillImportModel {
  busy: boolean;
  canImport: boolean;
  kind: ExternalSkillImportKind;
  label: string;
  tone: "default" | "success" | "warning";
}

export interface ExternalSkillListItemModel {
  avatarSeed: string;
  description: string;
  importState: ExternalSkillImportModel;
  installLabel: string;
  sourceLabel: string;
  sourceReference: string;
  title: string;
}

export interface ExternalSkillPreviewModel {
  avatarSeed: string;
  detailUrl: string;
  importState: ExternalSkillImportModel;
  item: ExternalSkillSearchItem;
  markdown: string;
  sourceLabel: string;
  subtitle: string;
  title: string;
}

type ExternalSkillLocalization = Pick<I18nContextValue, "t">;

interface ExternalSkillImportPresentation {
  canImport: boolean;
  kind: ExternalSkillImportKind;
  labelKey: TranslationKey;
  tone: ExternalSkillImportModel["tone"];
}

const IMPORT_PRESENTATIONS: Record<
  ExternalSkillImportKind,
  ExternalSkillImportPresentation
> = {
  available: {
    canImport: true,
    kind: "available",
    labelKey: "capability.skills_external_status_available",
    tone: "default",
  },
  conflict: {
    canImport: false,
    kind: "conflict",
    labelKey: "capability.skills_external_status_conflict",
    tone: "warning",
  },
  imported: {
    canImport: false,
    kind: "imported",
    labelKey: "capability.skills_external_status_imported",
    tone: "success",
  },
};

export function externalSkillKey(item: ExternalSkillSearchItem): string {
  return `${item.source_key || item.package_spec}@@${item.skill_slug}`;
}

export function isExternalSkillPreviewUnavailable(
  item: ExternalSkillSearchItem,
): boolean {
  return item.source_kind === "skills_sh"
    || item.import_mode === "skills_sh"
    || (item.source_kind === "private_registry" && !item.readme_markdown);
}

export function buildExternalSkillListItemModel(
  item: ExternalSkillSearchItem,
  importedSources: Map<string, Set<string>>,
  busyKeys: ReadonlySet<string>,
  localization: ExternalSkillLocalization,
): ExternalSkillListItemModel {
  const sourceLabel = externalSkillSourceLabel(item, localization);
  return {
    avatarSeed: item.skill_slug || item.name,
    description: item.description || item.readme_markdown || localization.t(
      "capability.skills_external_result_from_source",
      { source: sourceLabel },
    ),
    importState: buildExternalSkillImportModel(
      item,
      importedSources,
      busyKeys,
      localization,
    ),
    installLabel: item.source_key === "nexus_recommended"
      ? ""
      : localization.t("capability.skills_external_install_count", {
        count: formatInstallCount(item.installs),
      }),
    sourceLabel,
    sourceReference: externalSkillSourceReference(item),
    title: item.title || item.skill_slug,
  };
}

export function buildExternalSkillPreviewModel(
  item: ExternalSkillSearchItem | null,
  importedSources: Map<string, Set<string>>,
  busyKeys: ReadonlySet<string>,
  loading: boolean,
  localization: ExternalSkillLocalization,
): ExternalSkillPreviewModel | null {
  if (!item) return null;
  const listItem = buildExternalSkillListItemModel(
    item,
    importedSources,
    busyKeys,
    localization,
  );
  return {
    avatarSeed: listItem.avatarSeed,
    detailUrl: item.detail_url,
    importState: listItem.importState,
    item,
    markdown: buildPreviewMarkdown(item, loading, localization),
    sourceLabel: listItem.sourceLabel,
    subtitle: [listItem.sourceReference, listItem.installLabel].filter(Boolean).join(" · "),
    title: listItem.title,
  };
}

function buildExternalSkillImportModel(
  item: ExternalSkillSearchItem,
  importedSources: Map<string, Set<string>>,
  busyKeys: ReadonlySet<string>,
  localization: ExternalSkillLocalization,
): ExternalSkillImportModel {
  const kind = resolveExternalSkillImportKind(item, importedSources);
  const presentation = IMPORT_PRESENTATIONS[kind];
  return {
    busy: busyKeys.has(externalSkillKey(item)),
    canImport: presentation.canImport,
    kind: presentation.kind,
    label: localization.t(presentation.labelKey),
    tone: presentation.tone,
  };
}

function resolveExternalSkillImportKind(
  item: ExternalSkillSearchItem,
  importedSources: Map<string, Set<string>>,
): ExternalSkillImportKind {
  const sources = importedSources.get(item.skill_slug);
  if (!sources) return "available";
  return sources.has(item.package_spec) ? "imported" : "conflict";
}

function buildPreviewMarkdown(
  item: ExternalSkillSearchItem,
  loading: boolean,
  localization: ExternalSkillLocalization,
): string {
  if (loading && !item.readme_markdown) {
    return localization.t("capability.skills_external_preview_loading");
  }
  if (isExternalSkillPreviewUnavailable(item)) {
    return localization.t("capability.skills_external_preview_unavailable", {
      source: externalSkillSourceLabel(item, localization),
    });
  }
  return item.readme_markdown || item.description || localization.t(
    "capability.skills_external_preview_empty",
  );
}

function externalSkillSourceLabel(
  item: ExternalSkillSearchItem,
  localization: ExternalSkillLocalization,
): string {
  return item.source_name || item.source_kind || localization.t(
    "capability.skills_external_source_community",
  );
}

function externalSkillSourceReference(item: ExternalSkillSearchItem): string {
  return item.package_spec || item.git_url || item.raw_url || item.source;
}

function formatInstallCount(count: number): string {
  if (count < 1000) return `${count}`;
  return `${(count / 1000).toFixed(count >= 100000 ? 0 : 1)}K`;
}
