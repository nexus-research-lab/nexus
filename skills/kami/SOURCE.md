# 来源与许可证

本内置 Skill 完整引入 Tw93 的 [`Kami`](https://github.com/tw93/Kami) 轻量 Skill 包，并适配
Nexus 的内置 Skill 目录。

- 上游项目：https://github.com/tw93/Kami
- 固定来源提交：[`a211e7bbf9470493debfbbf0fe4645c05eaa546f`](https://github.com/tw93/Kami/commit/a211e7bbf9470493debfbbf0fe4645c05eaa546f)
- 固定来源目录：https://github.com/tw93/Kami/tree/a211e7bbf9470493debfbbf0fe4645c05eaa546f/plugins/kami/skills/kami
- 上游版本：1.11.0
- 上游许可证：MIT
- 上游版权：Copyright (c) 2026 Tw93

完整 MIT 许可证保留在 [LICENSE](LICENSE)。字体及可选渲染依赖见
[THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。

## Nexus 适配

- 增加 Nexus 目录元数据、中文触发说明、精选目录和 UI 描述；
- 将脚本与模板路径改为 `${CLAUDE_SKILL_DIR}`，生成物必须写入用户工作目录；
- 关闭内置 Skill 的静默更新检查与插件打包流程，更新只通过重新固定提交完成；
- 保留上游完整轻量 Skill 包：9 类文档/页面模板、18 类内联 SVG 图解、参考文档、内容
  schema、构建与验证脚本；
- 保留上游 MIT 许可证和 Source Han Serif K 的 OFL 文本，并补充 JetBrains Mono 的 OFL、
  商业中文字体、网络字体与 Python/系统依赖说明；
- 增加 `requirements.txt` 与 `requirements-optional.txt`，仅说明依赖，不自动安装。

上游 `scripts/tests/test_build.py` 是完整 Kami 仓库的维护者测试，会读取仓库网站、feed、插件
marketplace 与 `dist/kami.zip`；这些文件不属于轻量 Skill 包，因此该测试在 Nexus 内置目录中
不作为交付门禁。Nexus 使用自身的元数据、同步测试，并运行轻量包内可独立执行的模板检查。

本目录不会自动跟踪上游 `main`。升级时必须重新固定提交、核对模板和脚本差异、复核字体与
第三方许可证，再显式更新本文件。
