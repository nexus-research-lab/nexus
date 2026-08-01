# 第三方内容说明

本 Nexus 适配完整保留上游图标参考 `references/primitive-icons.md` 与画廊
`assets/icons.html`，并将上游 `scripts/vendor/icons/` 中 86 个原始 SVG 映射到
`assets/icons/`。上游第三方清单原文见
[THIRD_PARTY_LICENSES.upstream.md](THIRD_PARTY_LICENSES.upstream.md)，其中记录了 Tabler Icons
（MIT）、Simple Icons（CC0）、log-z/logos（MIT）、Devicon（MIT）及一次性来源图标。

上游图标参考还将 Apache Hop、Pentaho、Dagster 与 Stata 标为“使用前核验许可证”。这些
条目为完整保留上游内容而随 Skill 分发，原始警告未删除；上游项目的 MIT 许可证不自动授予
这些第三方品牌图标的权利。将相关图标用于公开、商业或再分发交付前，应单独核验授权，不能
核验时应替换为通用几何图标。

部分 HTML 模板通过网络请求 Google Fonts。这里仅保存字体名称与 CSS 链接，不捆绑字体
二进制；离线环境会使用模板中的系统字体回退。PNG 导出可选依赖 Playwright，Nexus 不会
自动安装。

产品名和商标仍归各自权利人所有。示例中出现相关名称仅用于技术说明，不代表认可、赞助或
关联。
