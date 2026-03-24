interface ContactsPlaceholderProps {
  agentId?: string;
}

export function ContactsPlaceholder({ agentId }: ContactsPlaceholderProps) {
  return (
    <div className="grid flex-1 gap-4 lg:grid-cols-[1fr_0.9fr]">
      <section className="workspace-card rounded-[28px] px-5 py-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
          Contacts List
        </p>
        <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700/60">
          下一阶段会把成员浏览、技能筛选和快速发起协作的入口迁到这里，避免 room 页面继续承担联系人管理职责。
        </p>
      </section>

      <aside className="workspace-card rounded-[28px] px-5 py-5">
        <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
          下一步
        </p>
        <ul className="mt-3 space-y-2 text-sm leading-6 text-slate-700/60">
          <li>成员列表与技能筛选</li>
          <li>成员 profile 页面</li>
          <li>发起 1v1 与邀请入 room</li>
          {agentId ? <li>优先展示成员资料：{agentId}</li> : null}
        </ul>
      </aside>
    </div>
  );
}
