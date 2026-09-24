# -*- coding: utf-8 -*-
"""RichPMS AI 通道 · WorkBuddy 一键浏览器授权（446）
流程：起本地回调(8372) → 自动弹系统浏览器到 RichPMS 授权页 → 用户输账密点同意
     → 回调收授权码 → 换令牌 → 回写 ~/.workbuddy/mcp.json（access + 30 天 refresh）
用法：python ai-workbuddy-auth.py   （环境变量 RICHPM_AI_BASE 覆盖后端地址，默认本地）
定位：WorkBuddy 技能包"令牌失效自愈"第二级 / 双击授权入口的浏览器体验版——
     密码只输在 RichPMS 官方授权页，不经过任何第三方脚本。
"""
import base64
import hashlib
import io
import json
import os
import secrets
import sys
import threading
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer

# Windows 控制台中文输出兜底（行缓冲：后台/管道场景输出实时可见）
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace", line_buffering=True)

def resolve_target():
    """GitBash 设 HOME 时优先（与 shell 脚本行为一致）；纯 Windows 环境回退 USERPROFILE"""
    home = os.environ.get("HOME")
    if home and os.path.isdir(home):
        return os.path.join(home, ".workbuddy", "mcp.json")
    return os.path.expanduser("~/.workbuddy/mcp.json")


TARGET = resolve_target()


def read_richpm_url():
    """读 mcp.json 中员工配置的 richpm 服务地址（授权 BASE 推导源）"""
    try:
        cfg = json.load(open(TARGET, encoding="utf-8"))
        found = []

        def walk(node):
            if isinstance(node, dict):
                for k, v in node.items():
                    if k == "richpm" and isinstance(v, dict) and v.get("url"):
                        found.append(v["url"])
                    else:
                        walk(v)
            elif isinstance(node, list):
                for item in node:
                    walk(item)

        walk(cfg)
        return found[0] if found else None
    except Exception:
        return None


def resolve_base():
    """后端地址优先级：RICHPM_AI_BASE 环境变量 > mcp.json 的 richpm.url 推导 > localhost 兜底
    （员工配置的 MCP 服务指向哪，授权就打哪——零配置自适应现网/测试环境）"""
    env = os.environ.get("RICHPM_AI_BASE")
    if env:
        return env.rstrip("/")
    url = read_richpm_url()
    if url:
        if "/api/ai/mcp" in url:
            return url.split("/api/ai/mcp")[0]
        parsed = urllib.parse.urlparse(url)
        if parsed.scheme and parsed.netloc:
            return f"{parsed.scheme}://{parsed.netloc}"
    return "http://localhost:8080"


BASE = resolve_base()
PORT = 8372
CLIENT_ID = "workbuddy"
REDIRECT = f"http://localhost:{PORT}/callback"
TIMEOUT_S = 300

state = {"code": None, "event": threading.Event()}


class CallbackHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path == "/callback":
            code = urllib.parse.parse_qs(parsed.query).get("code", [None])[0]
            body = (
                '<meta charset="utf-8"><body style="font-family:system-ui;background:#f0f5ff;'
                'display:flex;align-items:center;justify-content:center;height:100vh;margin:0">'
                '<div style="background:#fff;padding:40px 48px;border-radius:12px;'
                'box-shadow:0 8px 24px rgba(0,0,0,.08);text-align:center">'
                '<h2 style="color:#1e5eff;margin:0 0 8px">&#10024; RichPMS 授权成功</h2>'
                '<p style="color:#6b7280;margin:0">请关闭此页，回到 WorkBuddy 继续对话即可</p></div></body>'
            ).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            if code:
                state["code"] = code
                state["event"].set()
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, *args):
        pass  # 静默访问日志


def main():
    if not os.path.exists(TARGET):
        print(f"[提示] 未找到 {TARGET}——请先在 WorkBuddy 添加 richpm 服务器后再运行")
        sys.exit(2)
    verifier = secrets.token_urlsafe(48)[:64]
    challenge = base64.urlsafe_b64encode(
        hashlib.sha256(verifier.encode()).digest()).rstrip(b"=").decode()
    authorize_url = (
        f"{BASE}/api/ai/oauth/authorize?response_type=code&client_id={CLIENT_ID}"
        f"&redirect_uri={urllib.parse.quote(REDIRECT)}"
        f"&scope={urllib.parse.quote('report:read report:write')}"
        f"&state={secrets.token_hex(8)}&code_challenge={challenge}&code_challenge_method=S256"
    )
    try:
        srv = HTTPServer(("127.0.0.1", PORT), CallbackHandler)
    except OSError:
        print(f"[提示] 端口 {PORT} 被占用——请关闭上次未完成的授权窗口/脚本后重试")
        sys.exit(2)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    if os.environ.get("RICHPM_AI_NO_BROWSER") == "1":
        print(f"[手动打开] 浏览器未自动弹出，请复制以下地址到浏览器完成授权：\n{authorize_url}")
    else:
        webbrowser.open(authorize_url)
    print(f"[等待授权] 授权页已就绪（{TIMEOUT_S // 60} 分钟内有效）——"
          f"请在页面输入 PMS 账号密码完成授权")
    if not state["event"].wait(TIMEOUT_S):
        srv.shutdown()
        print("[超时] 未完成授权——可重新运行本脚本")
        sys.exit(2)
    srv.shutdown()

    # 授权码换令牌
    data = urllib.parse.urlencode({
        "grant_type": "authorization_code",
        "code": state["code"],
        "client_id": CLIENT_ID,
        "redirect_uri": REDIRECT,
        "code_verifier": verifier,
    }).encode()
    try:
        resp = json.loads(urllib.request.urlopen(
            urllib.request.Request(f"{BASE}/api/ai/oauth/token", data=data), timeout=15).read())
    except Exception as e:
        print(f"[失败] 换取令牌失败：{e}")
        sys.exit(2)
    token, refresh = resp.get("access_token"), resp.get("refresh_token")
    if not token:
        print("[失败] 服务端未返回令牌")
        sys.exit(2)

    # 回写 mcp.json（递归定位 richpm，其余配置原样保留）
    cfg = json.load(open(TARGET, encoding="utf-8"))
    updated = 0

    def walk(node):
        nonlocal updated
        if isinstance(node, dict):
            for k, v in node.items():
                if k == "richpm" and isinstance(v, dict):
                    v["headers"] = {"Authorization": f"Bearer {token}"}
                    if refresh:
                        v["x-richpm-refresh-token"] = refresh
                    updated += 1
                else:
                    walk(v)
        elif isinstance(node, list):
            for item in node:
                walk(item)

    walk(cfg)
    json.dump(cfg, open(TARGET, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
    print(f"[完成] 已回写 {updated} 处 richpm 令牌（有效期 {resp.get('expires_in')} 秒）——"
          f"回到 WorkBuddy 重连/新开会话即可使用")


if __name__ == "__main__":
    main()
