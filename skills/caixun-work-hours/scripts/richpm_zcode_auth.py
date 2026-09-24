# -*- coding: utf-8 -*-
"""RichPMS AI 通道 · ZCode 一键浏览器授权
流程：起本地回调(8372) → 自动弹系统浏览器到 RichPMS 授权页 → 用户输账密点同意
     → 回调收授权码 → 换令牌 → 回写 ~/.zcode/cli/config.json (access)
     + ~/.zcode/cli/richpm-auth.json (refresh, 30 天滑动)
参照技能包 scripts/ai-workbuddy-auth.py 的 PKCE 参数，仅目标改为 ZCode 配置。
用法：python richpm_zcode_auth.py  （环境变量 RICHPM_AI_BASE 可覆盖后端地址）
"""
import base64
import hashlib
import io
import json
import os
import secrets
import sys
import threading
import time
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace", line_buffering=True)

HOME = os.path.expanduser("~")
ZCODE_CONFIG = os.path.join(HOME, ".zcode", "cli", "config.json")
AUTH_STORE = os.path.join(HOME, ".zcode", "cli", "richpm-auth.json")

BASE = os.environ.get("RICHPM_AI_BASE", "http://192.168.7.30:8080").rstrip("/")
PORT = 8372
CLIENT_ID = "zcode"
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
                '<h2 style="color:#1e5eff;margin:0 0 8px">&#10024; 授权成功</h2>'
                '<p style="color:#6b7280;margin:0">请关闭此页，回到 ZCode 继续即可</p></div></body>'
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
        pass


def main():
    if not os.path.exists(ZCODE_CONFIG):
        print(f"[失败] 未找到 {ZCODE_CONFIG}")
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
        print(f"[手动打开] 请复制以下地址到浏览器完成授权：\n{authorize_url}")
    else:
        webbrowser.open(authorize_url)
    print(f"[等待授权] 授权页已就绪（{TIMEOUT_S // 60} 分钟内有效）——"
          f"请在页面输入 PMS 账号密码完成授权")
    if not state["event"].wait(TIMEOUT_S):
        srv.shutdown()
        print("[超时] 未完成授权——可重新运行本脚本")
        sys.exit(2)
    srv.shutdown()

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

    cfg = json.load(open(ZCODE_CONFIG, encoding="utf-8"))
    server = cfg["mcp"]["servers"]["richpm"]
    server["headers"] = {"Authorization": f"Bearer {token}"}
    json.dump(cfg, open(ZCODE_CONFIG, "w", encoding="utf-8"), ensure_ascii=False, indent=2)

    json.dump({
        "base": BASE,
        "refresh_token": refresh or "",
        "expires_at": int(time.time()) + int(resp.get("expires_in", 0)),
        "updated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
    }, open(AUTH_STORE, "w", encoding="utf-8"), ensure_ascii=False, indent=2)

    print(f"[完成] 令牌已写入 {ZCODE_CONFIG}")
    print(f"[完成] refresh token 已存入 {AUTH_STORE}（有效期 {resp.get('expires_in')} 秒）")


if __name__ == "__main__":
    main()
