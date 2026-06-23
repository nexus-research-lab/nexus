import type { StageWindowKind } from "../operation-desktop-types";

export function dock_icon_skin_for_kind(kind: StageWindowKind): string {
  if (kind === "finder") {
    return "border-[rgba(72,152,224,0.42)] bg-[linear-gradient(135deg,#5ac8fa_0%,#e8f5ff_48%,#ffffff_49%,#7dd3fc_100%)] text-[#14517a]";
  }
  if (kind === "browser") {
    return "border-[rgba(72,152,224,0.36)] bg-[radial-gradient(circle_at_50%_50%,#ffffff_0_24%,#5ac8fa_25%_52%,#2f6dff_53%_70%,#f45b69_71%_100%)] text-white";
  }
  if (kind === "terminal") {
    return "border-[rgba(141,224,173,0.32)] bg-[linear-gradient(135deg,#111827,#05080d)] text-[#8de0ad]";
  }
  if (kind === "code_editor") {
    return "border-[rgba(91,114,255,0.36)] bg-[linear-gradient(135deg,#243b74,#4f6fff)] text-white";
  }
  if (kind === "run_manifest" || kind === "evidence") {
    return "border-[rgba(117,131,149,0.30)] bg-[linear-gradient(135deg,#f8fafc,#cbd5e1)] text-[#334155]";
  }
  if (kind === "handoff") {
    return "border-[rgba(47,184,132,0.32)] bg-[linear-gradient(135deg,#f6fffb,#8de0ad_48%,#5b72ff)] text-[#123f3a]";
  }
  if (kind === "image_viewer" || kind === "markdown_reader" || kind === "pdf_reader" || kind === "word_reader") {
    return "border-[rgba(47,184,132,0.32)] bg-[linear-gradient(135deg,#ffffff,#a7f3d0_52%,#60a5fa)] text-[#17644f]";
  }
  if (kind === "permission_wait") {
    return "border-[rgba(117,131,149,0.34)] bg-[linear-gradient(135deg,#f8fafc,#e2e8f0)] text-[#475569]";
  }
  if (kind === "task_board") {
    return "border-[rgba(47,184,132,0.34)] bg-[linear-gradient(135deg,#08111f,#123f3a)] text-[#8de0ad]";
  }
  if (kind === "spreadsheet") {
    return "border-[rgba(47,184,132,0.34)] bg-[linear-gradient(135deg,#f0fdf4,#34d399)] text-[#064e3b]";
  }
  return "border-white/52 bg-white/44 text-(--icon-muted)";
}
