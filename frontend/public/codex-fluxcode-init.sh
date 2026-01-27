#!/usr/bin/env sh
set -eu

if [ -z "${OPENAI_API_KEY:-}" ]; then
  echo "[error] OPENAI_API_KEY is not set or empty. Please set OPENAI_API_KEY first." >&2
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
echo "Codex FluxCode initialization complete!"
echo "Config dir: $codex_dir"
echo "==========="
