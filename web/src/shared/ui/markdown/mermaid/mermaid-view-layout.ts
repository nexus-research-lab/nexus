const COMPACT_MAX_HEIGHT_CLASS_NAME = "max-h-[320px]";
const MARKDOWN_MAX_HEIGHT_CLASS_NAME = "max-h-[420px]";

export function getMermaidContainerClassName(
  compact: boolean,
  constrainHeight: boolean,
): string {
  if (compact) {
    return "my-2 max-h-[360px]";
  }
  return constrainHeight ? "my-3 max-h-[460px]" : "min-h-0";
}

export function getMermaidContentClassName(
  compact: boolean,
  constrainHeight: boolean,
): string {
  if (compact) {
    return COMPACT_MAX_HEIGHT_CLASS_NAME;
  }
  return constrainHeight ? MARKDOWN_MAX_HEIGHT_CLASS_NAME : "flex min-h-0 flex-1";
}

export function getMermaidBodyClassName(
  compact: boolean,
  constrainHeight: boolean,
): string {
  if (compact) {
    return COMPACT_MAX_HEIGHT_CLASS_NAME;
  }
  return constrainHeight ? MARKDOWN_MAX_HEIGHT_CLASS_NAME : "min-h-0 flex-1";
}

export function getMermaidSvgClassName(
  compact: boolean,
  constrainHeight: boolean,
): string {
  if (compact) {
    return "[&>svg]:!h-auto [&>svg]:!max-h-[288px] [&>svg]:!max-w-full [&>svg]:!w-auto";
  }
  if (constrainHeight) {
    return "[&>svg]:!h-auto [&>svg]:!max-h-[388px] [&>svg]:!max-w-full [&>svg]:!w-auto";
  }
  return "[&>svg]:!h-auto [&>svg]:!max-w-full [&>svg]:!w-auto";
}
