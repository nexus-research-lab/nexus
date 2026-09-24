# -*- coding: utf-8 -*-
"""RichPMS AI 通道 · MCP 服务一键配置（457 号起随技能包分发，461 号升级为校验修正）
用途：技能包自包含接入——检测本机已安装的 AI 客户端（WorkBuddy/Codex/Claude Code/ZCode/Cursor），
     保守合并写入 richpm MCP 服务器节点（不动其他任何配置），输出下一步授权指引。
用法：python setup_mcp.py   （RICHPM_AI_BASE 可覆盖后端地址，默认现网）
规则：目标节点已存在→校验 url，地址不符则修正（仅改 url，授权头等其余配置原样保留，改动前备份
     *.bak_richpm）；文件不存在→视为未安装该客户端（不创建文件，WorkBuddy 需先在设置界面
     启用一次 MCP 插件能力后再跑）。
"""
import io
import json
import os
import shutil
import sys

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace", line_buffering=True)

BASE = os.environ.get("RICHPM_AI_BASE", "http://192.168.7.30:8080").rstrip("/")
MCP_URL = f"{BASE}/api/ai/mcp"
HOME = os.environ.get("HOME") if os.environ.get("HOME") and os.path.isdir(os.environ.get("HOME")) \
    else os.path.expanduser("~")


def backup(path):
    """修改已有配置前备份（覆盖旧备份避免堆积）"""
    shutil.copy2(path, path + ".bak_richpm")


def save_json(path, cfg):
    json.dump(cfg, open(path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)


def setup_workbuddy():
    """~/.workbuddy/mcp.json 的 mcpServers.richpm（WorkBuddy 配置文件存在才写）"""
    path = os.path.join(HOME, ".workbuddy", "mcp.json")
    if not os.path.exists(path):
        return "skip（未检测到 WorkBuddy 配置）"
    cfg = json.load(open(path, encoding="utf-8"))
    servers = cfg.setdefault("mcpServers", {})
    node = servers.get("richpm")
    if node is None:
        servers["richpm"] = {"type": "http", "url": MCP_URL}
        save_json(path, cfg)
        return "ok（已写入 mcpServers.richpm）"
    if node.get("url") == MCP_URL:
        return "ok（已配置）"
    backup(path)
    node["url"] = MCP_URL
    save_json(path, cfg)
    return "ok（url 已修正，令牌等其余配置保留）"


def setup_codex():
    """~/.codex/config.toml 的 [mcp_servers.richpm]（tomllib 只读，文本手术；
    节点已存在→仅校验/修正段内 url 行，oauth 等其他键一律不动）"""
    path = os.path.join(HOME, ".codex", "config.toml")
    if not os.path.exists(path):
        return "skip（未检测到 Codex 配置）"
    s = open(path, encoding="utf-8").read()
    if "[mcp_servers.richpm]" not in s:
        with open(path, "a", encoding="utf-8") as f:
            f.write(f"\n[mcp_servers.richpm]\nurl = \"{MCP_URL}\"\n")
        return "ok（已追加 [mcp_servers.richpm]）"
    lines = s.splitlines(keepends=True)
    start = next(i for i, l in enumerate(lines) if l.strip().startswith("[mcp_servers.richpm]"))
    end = len(lines)
    for j in range(start + 1, len(lines)):
        if lines[j].lstrip().startswith("["):
            end = j
            break
    url_idx = next((j for j in range(start + 1, end) if lines[j].strip().startswith("url")), None)
    want = f'url = "{MCP_URL}"'
    if url_idx is not None and lines[url_idx].strip() == want:
        return "ok（已配置）"
    backup(path)
    if url_idx is not None:
        lines[url_idx] = want + ("\n" if lines[url_idx].endswith("\n") else "")
        open(path, "w", encoding="utf-8", newline="").write("".join(lines))
        return "ok（url 已修正，段内其余配置不动）"
    lines.insert(end, want + "\n")
    open(path, "w", encoding="utf-8", newline="").write("".join(lines))
    return "ok（已补 url 行，段内其余配置不动）"


def setup_claude():
    """~/.claude.json 顶层 mcpServers.richpm（Claude Code 用户级全局 MCP）"""
    path = os.path.join(HOME, ".claude.json")
    if not os.path.exists(path):
        return "skip（未检测到 Claude Code 配置）"
    cfg = json.load(open(path, encoding="utf-8"))
    servers = cfg.setdefault("mcpServers", {})
    node = servers.get("richpm")
    if node is None:
        servers["richpm"] = {"type": "http", "url": MCP_URL}
        save_json(path, cfg)
        return "ok（已写入 mcpServers.richpm）"
    if node.get("url") == MCP_URL:
        return "ok（已配置）"
    backup(path)
    node["url"] = MCP_URL
    save_json(path, cfg)
    return "ok（url 已修正，令牌等其余配置保留）"


def setup_zcode():
    """~/.zcode/cli/config.json 的 mcp.servers.richpm（ZCode 嵌套结构与他家不同）"""
    path = os.path.join(HOME, ".zcode", "cli", "config.json")
    if not os.path.exists(path):
        return "skip（未检测到 ZCode 配置）"
    cfg = json.load(open(path, encoding="utf-8"))
    servers = cfg.setdefault("mcp", {}).setdefault("servers", {})
    node = servers.get("richpm")
    if node is None:
        servers["richpm"] = {"type": "http", "url": MCP_URL}
        save_json(path, cfg)
        return "ok（已写入 mcp.servers.richpm）"
    if node.get("url") == MCP_URL:
        return "ok（已配置）"
    backup(path)
    node["url"] = MCP_URL
    save_json(path, cfg)
    return "ok（url 已修正，令牌等其余配置保留）"


def setup_cursor():
    """~/.cursor/mcp.json——检测目录而非文件（Cursor 装了但从未配过 MCP 时文件不存在，应创建）"""
    cursor_dir = os.path.join(HOME, ".cursor")
    path = os.path.join(cursor_dir, "mcp.json")
    if not os.path.isdir(cursor_dir):
        return "skip（未检测到 Cursor）"
    if os.path.exists(path):
        cfg = json.load(open(path, encoding="utf-8"))
    else:
        cfg = {}
    servers = cfg.setdefault("mcpServers", {})
    node = servers.get("richpm")
    if node is None:
        servers["richpm"] = {"url": MCP_URL}
        save_json(path, cfg)
        return "ok（已写入 mcpServers.richpm）"
    if node.get("url") == MCP_URL:
        return "ok（已配置）"
    backup(path)
    node["url"] = MCP_URL
    save_json(path, cfg)
    return "ok（url 已修正，令牌等其余配置保留）"


def main():
    print(f"[RichPMS] MCP 服务地址：{MCP_URL}")
    clients = [("WorkBuddy", setup_workbuddy), ("Codex", setup_codex), ("Claude Code", setup_claude),
               ("ZCode", setup_zcode), ("Cursor", setup_cursor)]
    results = []
    for name, fn in clients:
        try:
            results.append((name, fn()))
        except Exception as e:
            results.append((name, f"error（{e}）"))
    wrote = any(r.startswith(("ok（已写入", "ok（已追加", "ok（url 已修正", "ok（已补")) for _, r in results)
    print("[配置结果]")
    for name, r in results:
        print(f"  - {name}: {r}")
    if not wrote and all(r.startswith("skip") for _, r in results):
        print("\n[提示] 未检测到任何已安装的 AI 客户端——请先安装 WorkBuddy / Codex / Claude Code / ZCode / Cursor 之一再运行")
        sys.exit(2)
    print("\n[下一步] 授权（PMS 账号）：")
    print("  - WorkBuddy：运行 python scripts/ai-workbuddy-auth.py（弹浏览器输 PMS 账密）")
    print("  - ZCode：运行 python scripts/richpm_zcode_auth.py（弹浏览器输 PMS 账密）")
    print("  - Codex（CLI）：运行 codex mcp login richpm —— 自动动态注册并弹浏览器输 PMS 账密")
    print("    （Codex 桌面版对 http 内网授权服务器有安全限制，需 HTTPS/SSH 隧道，详见对接指导）")
    print("  - Claude Code：bash scripts/ai-login.sh（弹浏览器输 PMS 账密）")
    print("  完成后重启客户端/开新会话，说「查我的日报」验证接入")


if __name__ == "__main__":
    main()
