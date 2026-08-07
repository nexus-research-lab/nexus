/**
 * INPUT: 本批 Room 未读 Agent 回复的起点节点。
 * OUTPUT: 附着在该批次起点、文案居中的横线阅读边界。
 * POS: Room Feed 专属被动提示；不伪装成按钮，也不改变消息测量高度。
 */
export function GroupConversationUnreadMarker() {
  return (
    <div
      className="pointer-events-none absolute inset-x-2 top-0 z-10 flex h-4 -translate-y-1/2 items-center sm:inset-x-3"
      data-room-unread-marker
    >
      <span className="sr-only">未读消息从这里开始</span>
      <span
        aria-hidden="true"
        className="h-px flex-1 bg-[color:color-mix(in_srgb,var(--brand-action)_38%,var(--divider-subtle-color))]"
        data-room-unread-marker-line
      />
      <span
        aria-hidden="true"
        className="mx-3 shrink-0 text-[10px] font-medium leading-4 tracking-[0.08em] text-[color:color-mix(in_srgb,var(--brand-action)_72%,var(--text-soft))]"
      >
        新消息
      </span>
      <span
        aria-hidden="true"
        className="h-px flex-1 bg-[color:color-mix(in_srgb,var(--brand-action)_38%,var(--divider-subtle-color))]"
        data-room-unread-marker-line
      />
    </div>
  );
}
