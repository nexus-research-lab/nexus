export interface StageMinimizedWindowTile {
  aria_label: string;
  title: string;
}

export function buildStageMinimizedWindowTile({
  app_label,
  title,
}: {
  app_label: string;
  title: string;
}): StageMinimizedWindowTile {
  return {
    aria_label: `从 Dock 恢复：${title}`,
    title: `${app_label} · ${title} · 已最小化`,
  };
}
