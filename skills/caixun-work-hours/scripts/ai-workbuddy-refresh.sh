#!/usr/bin/env bash
# RichPMS AI 通道 · WorkBuddy 令牌续期（refresh 轮换，30 天滑动）
# 用途：access token 过期(401)时优先跑本脚本——用 mcp.json 里的 refresh_token 换新令牌，
#      无需账号密码；同时回写新的 refresh_token（旧串即废，30 天滚动）。
# 用法：RICHPM_AI_BASE=http://192.168.7.30:8080 bash ai-workbuddy-refresh.sh
# 退出码：0=续期成功；2=refresh 已失效（30 天静置或授权撤销/禁用/改密），须重跑 ai-workbuddy-login.sh
# 供 WorkBuddy 技能包调用：两级降级的第一级（失败时提示用户跑 richpm-auth.bat 重新授权）

set -e
TARGET="$HOME/.workbuddy/mcp.json"

# 一次读取 mcp.json：refresh token + richpm 服务地址（授权 BASE 推导源，零配置自适应）
PARSED=$(python - "$TARGET" <<'PYEOF'
import json, sys, os
path = os.path.expanduser(sys.argv[1])
try:
    cfg = json.load(open(path, encoding='utf-8'))
except Exception:
    print('\n'); raise SystemExit
def find(node):
    if isinstance(node, dict):
        for k, v in node.items():
            if k == 'richpm' and isinstance(v, dict):
                base = ''
                url = v.get('url') or ''
                if url:
                    if '/api/ai/mcp' in url:
                        base = url.split('/api/ai/mcp')[0]
                    elif url.startswith('http'):
                        base = url.split('/api/')[0]
                print(base)
                print(v.get('x-richpm-refresh-token') or '')
                return True
            if find(v): return True
    elif isinstance(node, list):
        for i in node:
            if find(i): return True
    return False
if not find(cfg):
    print('\n')
PYEOF
)
BASE_LINE=$(printf '%s\n' "$PARSED" | sed -n '1p')
REFRESH=$(printf '%s\n' "$PARSED" | sed -n '2p')
# BASE 优先级：环境变量 > mcp.json 推导 > localhost 兜底
BASE="${RICHPM_AI_BASE:-${BASE_LINE:-http://localhost:8080}}"

if [ -z "$REFRESH" ]; then
  echo "[提示] mcp.json 中没有 x-richpm-refresh-token——请先运行 richpm-auth.bat（或 ai-workbuddy-login.sh）完成首次授权"
  exit 2
fi

RESP=$(curl -s -X POST "$BASE/api/ai/oauth/token" \
  --data-urlencode "grant_type=refresh_token" \
  --data-urlencode "refresh_token=$REFRESH" \
  --data-urlencode "client_id=workbuddy") || true

TOKEN=$(printf '%s' "$RESP" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
if [ -z "$TOKEN" ]; then
  ERR=$(printf '%s' "$RESP" | grep -o '"error_description":"[^"]*"' | cut -d'"' -f4)
  echo "[需重新授权] refresh 已失效（${ERR:-网络异常或令牌无效}）——请运行 richpm-auth.bat，输入账号密码后即可恢复"
  exit 2
fi
NEW_REFRESH=$(printf '%s' "$RESP" | grep -o '"refresh_token":"[^"]*"' | cut -d'"' -f4)

python - "$TARGET" "$TOKEN" "$NEW_REFRESH" <<'PYEOF'
import json, sys, os
path, token, refresh = os.path.expanduser(sys.argv[1]), sys.argv[2], sys.argv[3]
cfg = json.load(open(path, encoding='utf-8'))
def walk(node):
    if isinstance(node, dict):
        for k, v in node.items():
            if k == 'richpm' and isinstance(v, dict):
                v['headers'] = {'Authorization': f'Bearer {token}'}
                if refresh:
                    v['x-richpm-refresh-token'] = refresh
            else:
                walk(v)
    elif isinstance(node, list):
        for item in node:
            walk(item)
walk(cfg)
json.dump(cfg, open(path, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
print('[完成] 令牌已续期并回写')
PYEOF
echo "下一步：WorkBuddy 重连 richpm / 开新会话即可用"