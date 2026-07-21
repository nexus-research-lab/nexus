import {
  Activity,
  CheckCircle2,
  Code2,
  Edit3,
  FileCode2,
  FileSpreadsheet,
  FileText,
  FolderTree,
  Globe2,
  ImageIcon,
  ListChecks,
  ListTree,
  Search,
  ShieldQuestion,
  PackageCheck,
  Terminal,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

import type {
  StageWindowKind,
} from "../operation-desktop-types";
import type { OperationKind } from "../operation-types";
export {
  isStageBackgroundWindow,
  positionForWindow,
} from "./operation-stage-window-position";
export { stageAppLabelForWindowKind } from "./operation-stage-app-identity";

export function iconForArtifactPath(path: string): LucideIcon {
  if (/\.(tsx?|jsx?|json|ya?ml|toml|css|scss|html?)$/i.test(path)) {
    return FileCode2;
  }
  if (/\.(csv|xlsx?|ods)$/i.test(path)) {
    return FileSpreadsheet;
  }
  if (/\.(png|jpe?g|webp|gif|svg)$/i.test(path)) {
    return ImageIcon;
  }
  return FileText;
}

export function iconForOperationKind(kind: OperationKind): LucideIcon {
  if (kind === "workspace_inspect") {
    return ListTree;
  }
  if (kind === "workspace_search") {
    return Search;
  }
  if (kind === "workspace_read") {
    return FileText;
  }
  if (kind === "workspace_edit" || kind === "artifact_update") {
    return Edit3;
  }
  if (kind === "command_run" || kind === "command_stop") {
    return Terminal;
  }
  if (kind === "web_research") {
    return Globe2;
  }
  if (kind === "task_delegate" || kind === "task_progress") {
    return Activity;
  }
  if (kind === "plan_update") {
    return Code2;
  }
  return CheckCircle2;
}

export function iconForWindowKind(kind: StageWindowKind): LucideIcon {
  if (kind === "finder") {
    return FolderTree;
  }
  if (kind === "terminal") {
    return Terminal;
  }
  if (kind === "browser") {
    return Globe2;
  }
  if (kind === "tasks") {
    return ListChecks;
  }
  if (kind === "run_manifest") {
    return ListChecks;
  }
  if (kind === "handoff") {
    return PackageCheck;
  }
  if (kind === "evidence") {
    return CheckCircle2;
  }
  if (kind === "permission_wait") {
    return ShieldQuestion;
  }
  if (kind === "spreadsheet") {
    return FileSpreadsheet;
  }
  if (kind === "image_viewer") {
    return ImageIcon;
  }
  if (kind === "code_editor") {
    return FileCode2;
  }
  if (kind === "file_preview") {
    return FileText;
  }
  return FileText;
}
