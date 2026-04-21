set -e

OUTPUT="${CLAUDE_TOOL_OUTPUT:-}"

ERROR_PATTERNS=(
  "error:"
  "Error:"
  "ERROR:"
  "failed"
  "FAILED"
  "command not found"
  "No such file"
  "Permission denied"
  "fatal:"
  "Exception"
  "Traceback"
  "SyntaxError"
  "TypeError"
  "exit code"
  "non-zero"
)

contains_error=false
for pattern in "${ERROR_PATTERNS[@]}"; do
  if [[ "$OUTPUT" == *"$pattern"* ]]; then
    contains_error=true
    break
  fi
done

if [ "$contains_error" = true ]; then
  cat << 'EOF'
<memory-error-reminder>
检测到命令失败。
如果这次失败有复用价值，请立刻把它记到今日日记：
- 类型：[ERR]
- 内容：错误现象、上下文、修复办法、是否可复现
- 目标：下次遇到同类问题时可以直接复用
</memory-error-reminder>
EOF
fi
