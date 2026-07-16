/** 只保留历史懒加载所需的布局哨兵，不向主 Feed 暴露内部加载状态。 */
export function ConversationRoundPlaceholder() {
  return (
    <div aria-hidden="true" className="h-20 w-full" />
  );
}
