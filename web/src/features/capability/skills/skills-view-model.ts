/**
 * =====================================================
 * @File   : skills-view-model.ts
 * @Date   : 2026-04-16 13:35
 * @Author : leemysw
 * 2026-04-16 13:35   Create
 * =====================================================
 */

import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
  SkillInfo,
} from "@/types/capability/skill";
import type { RefObject } from "react";

export type DiscoveryMode = "catalog" | "external";
export type SkillImportDialogMode = "local" | "git";

export interface SkillMarketplaceController {
  skills: SkillInfo[];
  searchQuery: string;
  discoveryMode: DiscoveryMode;
  activeCategory: string;
  externalQuery: string;
  externalSubmittedQuery: string;
  externalResults: ExternalSkillSearchItem[];
  externalSourceStatuses: ExternalSkillSourceStatus[];
  externalSources: ExternalSkillSourceInfo[];
  previewExternalItem: ExternalSkillSearchItem | null;
  externalLoading: boolean;
  externalPreviewLoading: boolean;
  sourceManagerOpen: boolean;
  sourceLoading: boolean;
  importDialogMode: SkillImportDialogMode | null;
  loading: boolean;
  busySkillName: string | null;
  busyExternalKey: string | null;
  statusMessage: string | null;
  errorMessage: string | null;
  fileInputRef: RefObject<HTMLInputElement | null>;
  categories: Array<{ key: string; label: string }>;
  visibleSkills: SkillInfo[];
  groupedSkills: Array<[string, SkillInfo[]]>;
  catalogCount: number;
  importedExternalSources: Map<string, Set<string>>;
  setSearchQuery: (value: string) => void;
  setDiscoveryMode: (value: DiscoveryMode) => void;
  setActiveCategory: (value: string) => void;
  setExternalQuery: (value: string) => void;
  setPreviewExternalItem: (value: ExternalSkillSearchItem | null) => void;
  setSourceManagerOpen: (value: boolean) => void;
  setImportDialogMode: (value: SkillImportDialogMode | null) => void;
  setStatusMessage: (value: string | null) => void;
  setErrorMessage: (value: string | null) => void;
  refreshMarketplace: () => Promise<void>;
  submitExternalSearch: () => void;
  handleUpdateSingle: (skillName: string) => Promise<void>;
  handleDeleteSkill: (skill: SkillInfo) => Promise<void>;
  handleUpdateInstalled: () => Promise<void>;
  handleLocalImport: (file: File) => Promise<void>;
  handleGitImport: (url: string, branch?: string, path?: string) => Promise<void>;
  handlePreviewExternal: (item: ExternalSkillSearchItem) => Promise<void>;
  handleImportExternal: (item: ExternalSkillSearchItem) => Promise<void>;
  refreshExternalSources: () => Promise<void>;
  handleToggleExternalSource: (
    source: ExternalSkillSourceInfo,
    enabled: boolean,
  ) => Promise<void>;
}
