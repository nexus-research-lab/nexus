# 第三方内容与依赖说明

## 字体

- `assets/fonts/JetBrainsMono.woff2` 来自上游 Kami 轻量包。JetBrains Mono 采用 SIL Open
  Font License 1.1；许可证与作者信息见
  `assets/fonts/LICENSE-JetBrainsMono.txt`，来源为 JetBrains Mono 官方仓库的 OFL 文本。
- Source Han Serif K 采用 SIL Open Font License 1.1；上游许可全文保留在
  `assets/fonts/LICENSE-SourceHanSerifK.txt`。轻量包不捆绑对应大型 OTF 文件。
- TsangerJinKai02 不随本内置 Skill 分发。上游模板和 `scripts/ensure-fonts.sh` 可从网络加载或
  下载它，但该字体仅允许个人免费使用，商业使用需向字体权利方取得许可。Nexus 不会自动
  下载；使用前必须由用户确认授权，不能确认时应改用 Source Han Serif、Noto Serif CJK、
  Songti SC 等已安装回退字体。
- Charter、YuMincho 和系统 CJK 回退字体不作为二进制随 Skill 分发，实际许可取决于用户
  系统或安装来源。

## 运行时依赖

- 核心脚本需要 Python 3.10+。
- PDF 构建需要 `weasyprint` 与 `pypdf`；视觉、字体、密度与孤行检查需要 `pymupdf`；代码
  高亮可选 `Pygments`。
- 可编辑 PPTX 可选 `python-pptx`；Marp 路径可选 `@marp-team/marp-cli`；不同平台还可能需要
  Cairo、Pango、Harfbuzz、fontconfig 或 LibreOffice。Nexus 不自动安装这些依赖。

网络字体、产品名和商标仍归各自权利人所有；模板中的引用不代表认可、赞助或关联。
