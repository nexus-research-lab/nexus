// INPUT: 已确定的候选、完整显示目录、非空回退名称与序号文案。
// OUTPUT: 保留原值/顺序的可区分选项，以及缺项当前值的禁用显示项。
// POS: 中立选择文字算法；不加载资源、不决定资格、不读取业务身份或语言 Store。

export interface NamedSelectionSource {
  value: string;
  label?: string | null;
  createdAt?: number;
}

export interface SelectionOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export function buildNamedSelectionOptions(
  candidates: readonly NamedSelectionSource[],
  copy: { fallbackLabel: string; numberedLabel: (name: string, number: number) => string },
  directory: readonly NamedSelectionSource[] = candidates,
): SelectionOption[] {
  // 辅助目录只稳定标签，不能扩大候选；空白目录名称不能抹掉领域已知名称。
  const identities = new Map(directory.map((item) => [item.value, item]));
  for (const item of candidates) {
    const known = identities.get(item.value);
    identities.set(item.value, {
      ...item,
      ...known,
      label: known?.label?.trim() ? known.label : item.label,
      createdAt: known?.createdAt ?? item.createdAt,
    });
  }
  const groups = new Map<string, NamedSelectionSource[]>();
  for (const item of identities.values()) {
    const key = (item.label?.trim() || copy.fallbackLabel).toLowerCase();
    const group = groups.get(key) ?? [];
    group.push(item);
    groups.set(key, group);
  }
  const labels = new Map<string, string>();
  for (const group of groups.values()) {
    const ordered = [...group].sort((a, b) => (a.createdAt ?? 0) - (b.createdAt ?? 0)
      || (a.value < b.value ? -1 : a.value > b.value ? 1 : 0));
    ordered.forEach((item, index) => {
      const label = item.label?.trim() || copy.fallbackLabel;
      labels.set(item.value, group.length > 1 || !item.label?.trim()
        ? copy.numberedLabel(label, index + 1)
        : label);
    });
  }
  return candidates.map((item) => ({ value: item.value, label: labels.get(item.value)! }));
}

export function includeUnavailableSelection(
  options: SelectionOption[],
  value: string,
  label: string,
): SelectionOption[] {
  if (!value || options.some((option) => option.value === value)) return options;
  return [{ value, label, disabled: true }, ...options];
}
