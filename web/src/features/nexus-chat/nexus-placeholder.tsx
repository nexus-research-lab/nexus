interface NexusPlaceholderProps {
  conversation_id?: string;
}

export function NexusPlaceholder({ conversation_id }: NexusPlaceholderProps) {
  return (
    <div className="grid flex-1 gap-4 lg:grid-cols-[1.25fr_0.75fr]">
      <section className="workspace-card rounded-[28px] px-5 py-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
          Nexus Chat
        </p>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700/60">
          下一阶段这里会落独立的 Nexus 对话页面，不再与 room 输入框模式混用。它会作为系统级管家，负责创建 room、邀请成员和整理协作网络。
        </p>
      </section>

      <aside className="workspace-card rounded-[28px] px-5 py-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
          下一步
        </p>
        <ul className="mt-3 space-y-2 text-sm leading-6 text-slate-700/60">
          <li>接入独立 Nexus controller</li>
          <li>建立系统动作历史与最近网络状态</li>
          <li>与 launcher 的 Ask Nexus 入口联通</li>
          {conversation_id ? <li>恢复指定 Nexus conversation：{conversation_id}</li> : null}
        </ul>
      </aside>
    </div>
  );
}
