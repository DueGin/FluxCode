#!/usr/bin/env sh
set -eu

if [ -z "${OPENAI_API_KEY:-}" ]; then
  echo "[错误] OPENAI_API_KEY 未设置或为空。请先设置环境变量 OPENAI_API_KEY。" >&2
  exit 1
fi

base_url="${FLUXCODE_BASE_URL:-https://flux-code.cc}"
base_url="${base_url%/}"

codex_dir="${HOME}/.codex"
auth_file="${codex_dir}/auth.json"
config_file="${codex_dir}/config.toml"

mkdir -p "$codex_dir"

printf '{"OPENAI_API_KEY":"%s"}' "$OPENAI_API_KEY" > "$auth_file"

cat > "$config_file" <<EOF
model_provider = "fluxcode"
model = "gpt-5.2-codex"
model_reasoning_effort = "medium"

[model_providers.fluxcode]
name = "fluxcode"
base_url = "$base_url"
wire_api = "responses"
requires_openai_auth = true
EOF

echo "==========="
echo "Codex FluxCode 初始化完成！"
echo "文件位置: $codex_dir"
echo "==========="

