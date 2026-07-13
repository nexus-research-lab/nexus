export interface StageKeyboardTargetDescriptor {
  content_editable?: string | null;
  is_content_editable?: boolean;
  tag_name?: string | null;
}

export function shouldIgnoreStageDesktopKeyboardTarget(
  target: StageKeyboardTargetDescriptor,
): boolean {
  if (target.is_content_editable || target.content_editable === "true") {
    return true;
  }
  const tag_name = target.tag_name?.toLowerCase();
  return tag_name === "input" || tag_name === "textarea" || tag_name === "select";
}
