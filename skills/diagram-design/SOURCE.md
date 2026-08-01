# 来源与许可证

本内置 Skill 基于 Cathryn Lavery 的
[`diagram-design`](https://github.com/cathrynlavery/diagram-design) 修改并适配 Nexus。

- 上游项目：https://github.com/cathrynlavery/diagram-design
- 固定来源提交：[`a157f7616473d966d6f433cf0b4d4f1880603504`](https://github.com/cathrynlavery/diagram-design/commit/a157f7616473d966d6f433cf0b4d4f1880603504)
- 固定来源目录：https://github.com/cathrynlavery/diagram-design/tree/a157f7616473d966d6f433cf0b4d4f1880603504/skills/diagram-design
- 上游版本：2.0
- 上游许可证：MIT
- 上游版权：Copyright (c) 2025 Cathryn Lavery

完整 MIT 许可证文本保留在 [LICENSE](LICENSE)。上游第三方归属原文保留在
[THIRD_PARTY_LICENSES.upstream.md](THIRD_PARTY_LICENSES.upstream.md)，Nexus 对完整图标库的分发
说明见 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。

## Nexus 适配

Nexus 在固定来源版本上进行了以下修改：

- 增加 Nexus 平台元数据、中文触发规则、目录分类和 UI 描述；
- 将品牌样式改为项目本地 `.diagram-design/style-guide.md`，禁止修改共享的内置 Skill；
- 增加 HTML 来源注释、无障碍和浏览器交付要求；
- 将上游 `scripts/lint-skin.py` 迁入 Skill，适配内置目录与项目本地样式指南；
- 明确与纯数值统计图表、`slide-maker` 的能力边界；
- 完整保留上游 `references/primitive-icons.md`、`assets/icons.html` 及图标画廊入口；将上游
  `scripts/vendor/icons/` 中 86 个原始 SVG 文件原样放入 `assets/icons/`；
- 保留 27 种图解参考、浅色/深色/长文/终端模板、示例画廊与完整上游图标库；
- 对上游标记为“使用前核验许可证”的图标保留原始警告，并在第三方说明中显式列出，不把
  上游 MIT 许可证误写成这些品牌图标的授权。

本目录不会自动跟踪上游 `main`。更新时必须重新固定提交、复核许可证与第三方素材，再显式
更新本文件中的来源版本。
