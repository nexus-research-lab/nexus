// INPUT: 用户输入的搜索词与业务声明的可搜索文本字段。
// OUTPUT: 稳定的 Unicode/大小写标准化结果，以及空查询直通的字段匹配器。
// POS: 全站客户端搜索语义真相；不持有 React 状态、资源筛选或服务端请求。

export type UiSearchField = string | number | null | undefined;
export type UiSearchMatchMode = "contains" | "prefix";

export interface UiSearchMatcher {
  empty: boolean;
  normalizedQuery: string;
  matches: (
    fields: readonly UiSearchField[],
    mode?: UiSearchMatchMode,
  ) => boolean;
}

export function normalizeUiSearchText(value: UiSearchField): string {
  return String(value ?? "")
    .normalize("NFKC")
    .trim()
    .toLowerCase();
}

export function createUiSearchMatcher(rawQuery: string): UiSearchMatcher {
  const normalizedQuery = normalizeUiSearchText(rawQuery);
  return {
    empty: normalizedQuery.length === 0,
    normalizedQuery,
    matches: (fields, mode = "contains") => normalizedQuery.length === 0
      || fields.some((field) => {
        const normalizedField = normalizeUiSearchText(field);
        return mode === "prefix"
          ? normalizedField.startsWith(normalizedQuery)
          : normalizedField.includes(normalizedQuery);
      }),
  };
}

export function matchesUiSearchFields(
  rawQuery: string,
  fields: readonly UiSearchField[],
  mode: UiSearchMatchMode = "contains",
): boolean {
  return createUiSearchMatcher(rawQuery).matches(fields, mode);
}
