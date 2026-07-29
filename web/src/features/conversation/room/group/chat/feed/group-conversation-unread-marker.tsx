/**
 * INPUT: 本批 Room 未读 Agent 回复的起点节点。
 * OUTPUT: 附着在该批次起点上的轻量新消息定位标记。
 * POS: Room Feed 专属视觉提示；不伪装成全局时间线分割线。
 */
export function GroupConversationUnreadMarker() {
  return (
    <div
      aria-label="本批新消息起点"
      className="pointer-events-none absolute left-1 top-0 z-10 flex h-5 -translate-y-[calc(100%+2px)] items-center gap-1.5 rounded-full border border-[color:color-mix(in_srgb,var(--brand-action)_16%,var(--surface-control-border))] bg-(--surface-control-background) px-2 text-[11px] font-medium tracking-[0.02em] text-(--brand-action) shadow-sm"
      data-room-unread-marker
    >
      <span
        aria-hidden="true"
        className="h-1.5 w-1.5 rounded-full bg-(--brand-action) shadow-[0_0_0_3px_color-mix(in_srgb,var(--brand)_10%,transparent)]"
      />
      <span>新消息</span>
    </div>
  );
}
