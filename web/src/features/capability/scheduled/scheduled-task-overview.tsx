interface ScheduledTaskOverviewProps {
  enabled: number;
  paused: number;
  running: number;
}

const METRIC_COPY = {
  enabled: { description: "后续继续参与调度", label: "已启用" },
  paused: { description: "暂时不会自动触发", label: "已暂停" },
  running: { description: "当前占用执行会话", label: "执行中" },
} as const;

function ScheduledMetricItem({
  description,
  label,
  value,
}: {
  description: string;
  label: string;
  value: number;
}) {
  return (
    <div className="min-w-0 py-3 md:px-4 md:first:pl-0 md:last:pr-0">
      <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-(--text-muted)">
        {label}
      </p>
      <div className="mt-1.5 flex items-baseline gap-2">
        <p className="text-[28px] font-semibold tracking-[-0.04em] text-(--text-strong)">
          {value}
        </p>
        <p className="min-w-0 truncate text-[12px] leading-5 text-(--text-muted)">
          {description}
        </p>
      </div>
    </div>
  );
}

export function ScheduledTaskOverview({
  enabled,
  paused,
  running,
}: ScheduledTaskOverviewProps) {
  const metrics = [
    { ...METRIC_COPY.running, value: running },
    { ...METRIC_COPY.enabled, value: enabled },
    { ...METRIC_COPY.paused, value: paused },
  ];
  return (
    <section className="mb-7 grid gap-0 divide-y divide-(--divider-subtle-color) border-b border-(--divider-subtle-color) pb-2 md:grid-cols-3 md:divide-x md:divide-y-0">
      {metrics.map((metric) => (
        <ScheduledMetricItem key={metric.label} {...metric} />
      ))}
    </section>
  );
}
