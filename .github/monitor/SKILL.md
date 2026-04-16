---
name: github-monitor
description: "GitHub 仓库巡检脚本说明。用于查看和执行仓库内的 GitHub PR / 分支监控脚本。"
author: leemysw
---

# GitHub Monitor

这个目录存放仓库内的 GitHub 监控辅助脚本，不属于 GitHub Actions workflow。

## 文件

- `config.json`：监控配置
- `monitor.sh`：执行一次 PR / 分支巡检
- `status.sh`：查看本地监控状态

## 使用方式

```bash
bash .github/monitor/status.sh
bash .github/monitor/monitor.sh
```

## 说明

- 脚本默认以仓库根目录为工作目录。
- 需要本地已安装 `git`、`gh`、`jq`、`codex`。
- 如需 Azure OpenAI Key，请通过环境变量注入，不在仓库中保存明文凭据。
