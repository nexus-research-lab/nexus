#!/usr/bin/env bash
# RichPMS AI 通道 · WorkBuddy 每日一签（access token 有效期由服务端 ai.token.access-seconds 决定）
# 用途：为 WorkBuddy 的 richpm MCP 配置重签访问令牌并回写 ~/.workbuddy/mcp.json
# 用法：RICHPM_AI_BASE=http://192.168.7.30:8080 bash ai-workbuddy-login.sh
#      （不设 RICHPM_AI_BASE 默认打本地 http://localhost:8080；GitBash 执行，密码不回显不落盘）
# 背景：WorkBuddy MCP 客户端不实现 OAuth 授权码流自动发起（435/436 实证），
#      token 到期仅报 401，须人工重签；本脚本一条命令完成 PKCE 授权+回写。
# 身份：client_id=workbuddy（203 号 SQL 专属预置，450 号起审计/治理按渠道归属）；
#      存量旧 refresh（claude-code 签）经本脚本续期会报 client_id 不匹配 → 按提示
#      重走一次授权即完成身份迁移（浏览器一键）。

set -e
TARGET="$HOME/.workbuddy/mcp.json"
# BASE 优先级：RICHPM_AI_BASE 环境变量 > mcp.json 的 richpm.url 推导 > localhost 兜底
# （员工配置的 MCP 服务指向哪，授权就打哪——零配置自适应现网/测试环境）
BASE_LINE=$(python - "$TARGET" <<'PYEOF' 2>/dev/null || echo ''
import json, sys, os
try:
    cfg = json.load(open(os.path.expanduser(sys.argv[1]), encoding='utf-8'))
except Exception:
    raise SystemExit
def find(node):
    if isinstance(node, dict):
        for k, v in node.items():
            if k == 'richpm' and isinstance(v, dict) and v.get('url'):
                url = v['url']
                print(url.split('/api/ai/mcp')[0] if '/api/ai/mcp' in url else url.split('/api/')[0])
                return True
            if find(v): return True
    elif isinstance(node, list):
        for i in node:
            if find(i): return True
    return False
find(cfg)
PYEOF
)
BASE="${RICHPM_AI_BASE:-${BASE_LINE:-http://localhost:8080}}"

read -rp "PMS 用户名/工号: " USERNAME
read -rsp "PMS 密码: " PASSWORD; echo

VERIFIER="wb$(date +%s%N)$(head -c6 /dev/urandom | od -An -tx1 | tr -d ' \n')verifier-pad-43chars-min"
CHALLENGE=$(printf '%s' "$VERIFIER" | openssl dgst -sha256 -binary | base64 | tr '+/' '-_' | tr -d '=')
REDIRECT="http://localhost:8372/callback"

LOC=$(curl -s -o /dev/null -w "%{redirect_url}" -X POST "$BASE/api/ai/oauth/authorize" \
  --data-urlencode "response_type=code" \
  --data-urlencode "client_id=workbuddy" \
  --data-urlencode "redirect_uri=$REDIRECT" \
  --data-urlencode "scope=report:read report:write" \
  --data-urlencode "code_challenge=$CHALLENGE" \
  --data-urlencode "code_challenge_method=S256" \
  --data-urlencode "username=$USERNAME" \
  --data-urlencode "password=$PASSWORD")

CODE=$(printf '%s' "$LOC" | sed -n 's/.*code=\([^&]*\).*/\1/p')
if [ -z "$CODE" ]; then
  echo "[失败] 未取得授权码——请核对账号密码与 BASE 地址（$BASE）"
  exit 1
fi

RESP=$(curl -s -X POST "$BASE/api/ai/oauth/token" \
  --data-urlencode "grant_type=authorization_code" \
  --data-urlencode "code=$CODE" \
  --data-urlencode "client_id=workbuddy" \
  --data-urlencode "redirect_uri=$REDIRECT" \
  --data-urlencode "code_verifier=$VERIFIER")
TOKEN=$(printf '%s' "$RESP" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
REFRESH=$(printf '%s' "$RESP" | grep -o '"refresh_token":"[^"]*"' | cut -d'"' -f4)
EXPIRES=$(printf '%s' "$RESP" | grep -o '"expires_in":[0-9]*' | cut -d: -f2)
if [ -z "$TOKEN" ]; then echo "[失败] 换取令牌失败"; exit 1; fi

# 回写 ~/.workbuddy/mcp.json：递归定位 richpm 节点更新 headers（不假设文件结构，其余配置原样保留）
python - "$TARGET" "$TOKEN" "$REFRESH" <<'PYEOF'
import json, sys, os
path, token, refresh = sys.argv[1], sys.argv[2], sys.argv[3]
path = os.path.expanduser(path)
if not os.path.exists(path):
    print(f'[失败] 未找到 {path}——请先在 WorkBuddy 添加 richpm 服务器后再运行本脚本')
    sys.exit(1)
cfg = json.load(open(path, encoding='utf-8'))
updated = 0
def walk(node):
    global updated
    if isinstance(node, dict):
        for k, v in node.items():
            if k == 'richpm' and isinstance(v, dict):
                v['headers'] = {'Authorization': f'Bearer {token}'}
                if refresh:
                    v['x-richpm-refresh-token'] = refresh  # 30 天续期凭据（WorkBuddy 技能包 401 自动续期用）
                updated += 1
            else:
                walk(v)
    elif isinstance(node, list):
        for item in node:
            walk(item)
walk(cfg)
json.dump(cfg, open(path, 'w', encoding='utf-8'), ensure_ascii=False, indent=2)
print(f'[完成] 已刷新 {updated} 处 richpm 令牌')
PYEOF

echo "有效期 ${EXPIRES} 秒。下一步：WorkBuddy 设置 → MCP 连接器 → 重连/启用 richpm，再开新会话即可用"