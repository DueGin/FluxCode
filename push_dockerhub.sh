#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
用法：
  ./push_dockerhub.sh --target <dockerhub_user>/<repo>[:tag] [选项]
  ./push_dockerhub.sh <dockerhub_user>/<repo>[:tag] [选项]

常用选项：
  --source <local_image[:tag]>   本地已构建镜像（默认：fluxcode:latest）
  --build                        构建本地镜像后再推送（默认行为，使用 Dockerfile）
  --auto-build                   仅当本地镜像不存在时才构建
  --no-build                     不构建（要求本地镜像已存在）
  --single-arch                  关闭 buildx 多架构，使用本机架构构建+推送
  --env-file <path>              从 env 文件读取 FLUXCODE_IMAGE 作为 target（例如 deploy/.env）
  --login                        推送前执行 docker login（可交互或使用 token）
  --platforms <list>             使用 buildx 多架构构建并直接推送（默认：linux/amd64,linux/arm64）
  --builder <name>               buildx builder 名称（默认：fluxcode-builder）

环境变量（可选）：
  DOCKERHUB_USERNAME / DOCKERHUB_TOKEN   配合 --login 可无交互登录：
    DOCKERHUB_TOKEN 建议使用 Docker Hub Access Token
  GOPROXY / GOSUMDB                      构建时传给 Dockerfile（默认与 build_image.sh 一致）
  BUILDER_NAME                           等同 --builder（默认：fluxcode-builder）
  APK_MIRROR / APK_MIRROR_FALLBACK        Alpine apk 镜像（默认与 Dockerfile 一致）

示例：
  # 1) 一键多架构构建并推送（默认 linux/amd64,linux/arm64）
  ./push_dockerhub.sh --target duegin/fluxcode:latest --login

  # 2) 从 deploy/.env 读取 FLUXCODE_IMAGE 作为目标镜像名，然后推送
  ./push_dockerhub.sh --env-file deploy/.env --login

  # 3) 自定义平台列表
  ./push_dockerhub.sh duegin/fluxcode:latest --platforms linux/amd64,linux/arm64,linux/arm/v7 --login

  # 4) 仅构建/推送本机架构（不使用 buildx）
  ./push_dockerhub.sh duegin/fluxcode:latest --single-arch --login
EOF
}

die() {
  echo "错误：$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "未找到命令：$1"
}

read_env_key() {
  local env_file="$1"
  local key="$2"
  [[ -f "$env_file" ]] || return 1
  local line
  line="$(grep -E "^[[:space:]]*${key}=" "$env_file" | head -n 1 || true)"
  [[ -n "$line" ]] || return 1
  local value="${line#*=}"
  value="${value%$'\r'}"
  value="${value#\"}"; value="${value%\"}"
  value="${value#\'}"; value="${value%\'}"
  echo "$value"
}

SOURCE_IMAGE="${SOURCE_IMAGE:-fluxcode:latest}"
TARGET_IMAGE="${TARGET_IMAGE:-}"
ENV_FILE="${ENV_FILE:-}"
BUILD_MODE="always" # auto|always|never
LOGIN=0
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}"
BUILDER_NAME="${BUILDER_NAME:-fluxcode-builder}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source)
      SOURCE_IMAGE="${2:-}"; shift 2 ;;
    --target)
      TARGET_IMAGE="${2:-}"; shift 2 ;;
    --env-file)
      ENV_FILE="${2:-}"; shift 2 ;;
    --build)
      BUILD_MODE="always"; shift ;;
    --auto-build)
      BUILD_MODE="auto"; shift ;;
    --no-build)
      BUILD_MODE="never"; shift ;;
    --single-arch)
      PLATFORMS=""; shift ;;
    --login)
      LOGIN=1; shift ;;
    --platforms)
      PLATFORMS="${2:-}"; BUILD_MODE="always"; shift 2 ;;
    --builder)
      BUILDER_NAME="${2:-}"; shift 2 ;;
    -h|--help)
      usage; exit 0 ;;
    *)
      if [[ -z "$TARGET_IMAGE" ]]; then
        TARGET_IMAGE="$1"; shift
      else
        die "未知参数：$1（使用 --help 查看用法）"
      fi
      ;;
  esac
done

need_cmd docker
docker info >/dev/null 2>&1 || die "Docker daemon 未运行（请先启动 Docker）"

get_buildx_driver() {
  # 从 buildx inspect 输出中解析 driver（docker/docker-container/...）
  # 兼容：Driver: docker
  docker buildx inspect "$@" 2>/dev/null | sed -n 's/^[[:space:]]*Driver:[[:space:]]*//p' | head -n 1
}

ensure_multiarch_builder() {
  local desired_name="$1"
  local active_name="$desired_name"
  local desired_driver
  local active_driver

  # 如果 desired builder 存在但 driver=docker（不支持多平台），则改用一个 docker-container builder
  if docker buildx inspect "$desired_name" >/dev/null 2>&1; then
    desired_driver="$(get_buildx_driver "$desired_name" || true)"
    if [[ "$desired_driver" == "docker" ]]; then
      active_name="${desired_name}-container"
    fi
  fi

  if docker buildx inspect "$active_name" >/dev/null 2>&1; then
    docker buildx use "$active_name" >/dev/null 2>&1 || true
  else
    # 明确使用 docker-container driver，避免落到 docker driver 导致多架构失败
    docker buildx create --name "$active_name" --driver docker-container --use >/dev/null
  fi

  active_driver="$(get_buildx_driver "$active_name" || true)"
  if [[ "$active_driver" == "docker" ]]; then
    die "当前 buildx builder driver= docker，无法多架构构建；请改用 --single-arch，或在 Docker Desktop 启用 containerd image store。"
  fi

  # 启动/预热 builder（会拉起 buildkit 容器）
  docker buildx inspect --bootstrap >/dev/null 2>&1 || true
}

if [[ -z "$TARGET_IMAGE" && -n "$ENV_FILE" ]]; then
  TARGET_IMAGE="$(read_env_key "$ENV_FILE" "FLUXCODE_IMAGE" || true)"
fi
[[ -n "$TARGET_IMAGE" ]] || die "未指定目标镜像（使用 --target 或 --env-file，或直接传入目标镜像名）"

# 如果 target 未显式包含 tag，则默认补 :latest
if [[ "${TARGET_IMAGE##*/}" != *:* ]]; then
  TARGET_IMAGE="${TARGET_IMAGE}:latest"
fi

if [[ "$LOGIN" -eq 1 ]]; then
  if [[ -n "${DOCKERHUB_USERNAME:-}" && -n "${DOCKERHUB_TOKEN:-}" ]]; then
    echo "$DOCKERHUB_TOKEN" | docker login -u "$DOCKERHUB_USERNAME" --password-stdin
  else
    docker login
  fi
fi

# 多架构：直接 buildx --push，跳过 tag/push 流程
if [[ -n "$PLATFORMS" ]]; then
  docker buildx version >/dev/null 2>&1 || die "未检测到 docker buildx（请升级 Docker 或启用 buildx；或使用 --single-arch）"
  ensure_multiarch_builder "$BUILDER_NAME"
  docker buildx build \
    --platform "$PLATFORMS" \
    -t "$TARGET_IMAGE" \
    --push \
    -f Dockerfile \
    --build-arg GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" \
    --build-arg GOSUMDB="${GOSUMDB:-sum.golang.google.cn}" \
    --build-arg APK_MIRROR="${APK_MIRROR:-https://mirrors.aliyun.com/alpine}" \
    --build-arg APK_MIRROR_FALLBACK="${APK_MIRROR_FALLBACK:-https://dl-cdn.alpinelinux.org/alpine}" \
    .
  echo "已构建并推送（多架构）：$TARGET_IMAGE"
  exit 0
fi

source_exists=0
if docker image inspect "$SOURCE_IMAGE" >/dev/null 2>&1; then
  source_exists=1
fi

if [[ "$BUILD_MODE" == "always" || ( "$BUILD_MODE" == "auto" && "$source_exists" -eq 0 ) ]]; then
  echo "开始构建本地镜像：$SOURCE_IMAGE"
  docker build -t "$SOURCE_IMAGE" \
    -f Dockerfile \
    --build-arg GOPROXY="${GOPROXY:-https://goproxy.cn,direct}" \
    --build-arg GOSUMDB="${GOSUMDB:-sum.golang.google.cn}" \
    --build-arg APK_MIRROR="${APK_MIRROR:-https://mirrors.aliyun.com/alpine}" \
    --build-arg APK_MIRROR_FALLBACK="${APK_MIRROR_FALLBACK:-https://dl-cdn.alpinelinux.org/alpine}" \
    .
fi

docker image inspect "$SOURCE_IMAGE" >/dev/null 2>&1 || die "找不到本地镜像：$SOURCE_IMAGE（可用 --build 构建）"

echo "打 tag：$SOURCE_IMAGE -> $TARGET_IMAGE"
docker tag "$SOURCE_IMAGE" "$TARGET_IMAGE"

echo "开始推送：$TARGET_IMAGE"
docker push "$TARGET_IMAGE"

echo "推送完成：$TARGET_IMAGE"
